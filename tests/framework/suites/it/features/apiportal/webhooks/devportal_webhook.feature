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

@devportal-webhook @APIPORTAL
Feature: Credentials issued in the API Portal authorize gateway invocation via webhook
  As an API platform operator running the full product suite
  I want an API key and subscription created in the API Portal to reach the gateway
  through the signed webhook to platform-api
  So that a consumer using developer-portal-issued credentials can invoke the API end to end

  @APIP-053 @lifecycle @smoke
  Scenario: API keys and subscriptions issued in the API Portal reach the gateway through their whole lifecycle
    # A developer-portal API key and subscription propagate to the gateway and authorize a call
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And a webhook subscriber targeting the control plane is registered in the API Portal
    And I generate a unique resource name from "devportal-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane
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

    When I set header "API-Key" to "${CTX:apiKeyValue}"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    And I send a "GET" request to "${CTX:apiContext}/" until status 200

    When I clear all headers
    And I send a "GET" request to "${CTX:apiContext}/"
    Then the response status code should be 401

    # Developer-portal credential-lifecycle changes propagate to platform-api and the gateway
    Given I generate a unique resource name from "devportal-plan" and store it as "planHandle"
    And I create a subscription plan "${CTX:planHandle}" allowing 10000 requests per hour via the control plane
    And I generate a unique resource name from "devportal-plan2" and store it as "plan2Handle"
    And I create a subscription plan "${CTX:plan2Handle}" allowing 5000 requests per hour via the control plane
    And I generate a unique resource name from "devportal-api" and store it as "apiHandle"
    And I generate a unique API context from "/devportal" and store it as "apiContext"
    When I create a secured REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" offering plan "${CTX:planHandle}"
    And I deploy the "RestApi" "${CTX:apiHandle}" to the gateway via the control plane
    Then I send a "GET" request to "${CTX:apiContext}/" until status 401

    When the subscription plan "${CTX:planHandle}" is synced to the API Portal
    And the subscription plan "${CTX:plan2Handle}" is synced to the API Portal
    And the "${CTX:apiHandle}" API is published to the API Portal offering plan "${CTX:planHandle},${CTX:plan2Handle}"
    And an application subscribed to API "${CTX:apiHandle}" plan "${CTX:planHandle}" is created in the API Portal, the token is stored as "subscriptionToken" and the subscription is stored as "subscriptionId"
    And an API key for API "${CTX:apiHandle}" is generated in the API Portal, the value is stored as "apiKeyValue" and the handle is stored as "apiKeyHandle"

    When I set header "API-Key" to "${CTX:apiKeyValue}"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    And I send a "GET" request to "${CTX:apiContext}/" until status 200

    # --- API key lifecycle: the subscription stays ACTIVE throughout, so a rejection (401)
    # isolates the key. ---
    When the API key "${CTX:apiKeyHandle}" for API "${CTX:apiHandle}" is regenerated with expiry "2020-01-01T00:00:00Z" in the API Portal and stored as "apiKeyValue"
    And I set header "API-Key" to "${CTX:apiKeyValue}"
    Then I send a "GET" request to "${CTX:apiContext}/" until status 401

    When the API key "${CTX:apiKeyHandle}" for API "${CTX:apiHandle}" is regenerated with expiry "2035-01-01T00:00:00Z" in the API Portal and stored as "apiKeyValue"
    And I set header "API-Key" to "${CTX:apiKeyValue}"
    Then I send a "GET" request to "${CTX:apiContext}/" until status 200

    When the API key "${CTX:apiKeyHandle}" for API "${CTX:apiHandle}" is revoked in the API Portal
    Then I send a "GET" request to "${CTX:apiContext}/" until status 401

    When an API key for API "${CTX:apiHandle}" is generated in the API Portal, the value is stored as "apiKeyValue" and the handle is stored as "apiKeyHandle"
    And I set header "API-Key" to "${CTX:apiKeyValue}"
    Then I send a "GET" request to "${CTX:apiContext}/" until status 200

    # --- Subscription lifecycle: the API key stays valid throughout, so a rejection (403)
    # isolates the subscription. ---
    When the subscription "${CTX:subscriptionId}" is switched to plan "${CTX:plan2Handle}" in the API Portal
    Then platform-api reports the subscription for API "${CTX:apiHandle}" using plan "${CTX:plan2Handle}"
    And I send a "GET" request to "${CTX:apiContext}/" until status 200

    When the subscription "${CTX:subscriptionId}" token is regenerated in the API Portal, the previous token is stored as "prevSubscriptionToken" and the new token is stored as "subscriptionToken"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    Then I send a "GET" request to "${CTX:apiContext}/" until status 200

    When I set header "Subscription-Key" to "${CTX:prevSubscriptionToken}"
    Then I send a "GET" request to "${CTX:apiContext}/" until status 401 or 403

    When the subscription "${CTX:subscriptionId}" status is set to "INACTIVE" in the API Portal
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    Then I send a "GET" request to "${CTX:apiContext}/" until status 403

    When the subscription "${CTX:subscriptionId}" status is set to "ACTIVE" in the API Portal
    Then I send a "GET" request to "${CTX:apiContext}/" until status 200

    When the subscription "${CTX:subscriptionId}" is removed in the API Portal
    Then I send a "GET" request to "${CTX:apiContext}/" until status 403

    # A deprecated API keeps serving its existing subscription and API key at the gateway
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
    When I set header "API-Key" to "${CTX:apiKeyValue}"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    Then I send a "GET" request to "${CTX:apiContext}/" until status 200

    When I send an authenticated API Portal "GET" request to "/apis/dp-${CTX:apiHandle}" as "admin"
    Then the response status code should be 200
    And the JSON response field "status" should be "PUBLISHED"
    And I store the JSON response field "name" as "portalApiName"
    And I store the JSON response field "version" as "portalApiVersion"
    When I update the API Portal API "dp-${CTX:apiHandle}" from "resources/templates/api-portal-api.yaml" as "admin" with values:
      | id      | dp-${CTX:apiHandle}     |
      | name    | ${CTX:portalApiName}    |
      | version | ${CTX:portalApiVersion} |
      | status  | DEPRECATED              |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/dp-${CTX:apiHandle}" as "admin"
    Then the JSON response field "status" should be "DEPRECATED"
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}" as "admin"
    Then the JSON response field "status" should be "ACTIVE"

    When I set header "API-Key" to "${CTX:apiKeyValue}"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    And I send a "GET" request to "${CTX:apiContext}/"
    Then the response status code should be 200
    When I clear all headers
    And I send a "GET" request to "${CTX:apiContext}/"
    Then the response status code should be 401

  @APIP-053 @type:negative @type:security
  Scenario: The control plane processes only signed, recent, known API Portal events, and each one once
    # The control plane processes only API Portal events signed with its secret within the replay window
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And a webhook subscriber targeting the control plane is registered in the API Portal
    And I generate a unique value from "devportal-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane
    Given I generate a unique resource name from "receiver-signed-meter" and store it as "signedMeter"
    And I generate a unique resource name from "receiver-unsigned-meter" and store it as "unsignedMeter"
    And I generate a unique resource name from "receiver-forged-meter" and store it as "forgedMeter"
    And I generate a unique resource name from "receiver-stale-meter" and store it as "staleMeter"
    And the control plane meter "${CTX:unsignedMeter}" is registered for cleanup
    And the control plane meter "${CTX:forgedMeter}" is registered for cleanup
    And the control plane meter "${CTX:staleMeter}" is registered for cleanup
    When the API Portal sends the "meter.created" event to the control plane with data:
      """
      {"handle":"${CTX:signedMeter}","display_name":"Signed ${CTX:signedMeter}","type":"REQUEST_COUNT","filter_fields":[{"field":"Status","event_key":"response.status"}]}
      """
    Then the response status code should be 200
    And the JSON response field "status" should be "processed"
    When the API Portal sends the "meter.created" event to the control plane unsigned with data:
      """
      {"handle":"${CTX:unsignedMeter}","display_name":"Unsigned ${CTX:unsignedMeter}","type":"REQUEST_COUNT","filter_fields":[{"field":"Status","event_key":"response.status"}]}
      """
    Then the response status code should be 401
    And the JSON response field "code" should be "UNAUTHORIZED"
    And the response body should not contain "signature"
    When the API Portal sends the "meter.created" event to the control plane signed with another secret with data:
      """
      {"handle":"${CTX:forgedMeter}","display_name":"Forged ${CTX:forgedMeter}","type":"REQUEST_COUNT","filter_fields":[{"field":"Status","event_key":"response.status"}]}
      """
    Then the response status code should be 401
    And the JSON response field "code" should be "UNAUTHORIZED"
    And the response body should not contain "signature"
    When the API Portal sends the "meter.created" event to the control plane with a stale signature with data:
      """
      {"handle":"${CTX:staleMeter}","display_name":"Stale ${CTX:staleMeter}","type":"REQUEST_COUNT","filter_fields":[{"field":"Status","event_key":"response.status"}]}
      """
    Then the response status code should be 401
    And the JSON response field "code" should be "UNAUTHORIZED"
    And the response body should not contain "tolerance"
    When I send a "GET" request to the control plane at "/meters/${CTX:signedMeter}"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Signed ${CTX:signedMeter}"
    When I send a "GET" request to the control plane at "/meters/${CTX:unsignedMeter}"
    Then the response status code should be 404
    When I send a "GET" request to the control plane at "/meters/${CTX:forgedMeter}"
    Then the response status code should be 404
    When I send a "GET" request to the control plane at "/meters/${CTX:staleMeter}"
    Then the response status code should be 404

    # An API Portal event of an unknown type or for an unknown organization is refused and changes nothing
    # The receiver answers every failed delivery alike; the real reason appears only in its log.
    Given I generate a unique resource name from "receiver-known-meter" and store it as "knownMeter"
    And I generate a unique resource name from "receiver-untyped-meter" and store it as "untypedMeter"
    And I generate a unique resource name from "receiver-foreign-meter" and store it as "foreignMeter"
    And I generate a unique value from "receiver-missing-org" and store it as "missingOrg"
    When the API Portal sends the "meter.archived" event to the control plane with data:
      """
      {"handle":"${CTX:untypedMeter}","display_name":"Untyped ${CTX:untypedMeter}","type":"REQUEST_COUNT","filter_fields":[{"field":"Status","event_key":"response.status"}]}
      """
    Then the response status code should be 401
    And the JSON response field "code" should be "UNAUTHORIZED"
    When the API Portal sends the "meter.created" event for organization "${CTX:missingOrg}" to the control plane with data:
      """
      {"handle":"${CTX:foreignMeter}","display_name":"Foreign ${CTX:foreignMeter}","type":"REQUEST_COUNT","filter_fields":[{"field":"Status","event_key":"response.status"}]}
      """
    Then the response status code should be 401
    And the JSON response field "code" should be "UNAUTHORIZED"
    When the API Portal sends the "meter.created" event to the control plane with data:
      """
      {"handle":"${CTX:knownMeter}","display_name":"Known ${CTX:knownMeter}","type":"REQUEST_COUNT","filter_fields":[{"field":"Status","event_key":"response.status"}]}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/meters/${CTX:knownMeter}"
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/meters/${CTX:untypedMeter}"
    Then the response status code should be 404
    When I send a "GET" request to the control plane at "/meters/${CTX:foreignMeter}"
    Then the response status code should be 404

    # The same API Portal event delivered twice is processed once
    Given I generate a unique resource name from "receiver-replayed-meter" and store it as "meterId"
    And the control plane meter "${CTX:meterId}" is registered for cleanup
    When the API Portal sends the "meter.created" event to the control plane signed with data:
      """
      {"handle":"${CTX:meterId}","display_name":"Replayed ${CTX:meterId}","type":"REQUEST_COUNT","filter_fields":[{"field":"Status","event_key":"response.status"}]}
      """
    Then the response status code should be 200
    When the API Portal sends its last event to the control plane again
    Then the response status code should be 200
    And the JSON response field "status" should be "processed"
    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Replayed ${CTX:meterId}"
    When I send a "GET" request to the control plane at "/meters?query=${CTX:meterId}"
    Then the response status code should be 200
    And the JSON response array field "list" should have 1 item
