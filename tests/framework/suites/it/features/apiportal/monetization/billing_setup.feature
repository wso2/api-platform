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

@api-portal-billing-setup
Feature: Setting up billing in the API Portal
  As an API Portal administrator
  I want to connect Moesif and Stripe with my keys, and remove them again
  So that paid plans can be sold, and nothing made with my keys is left behind when I stop

  Background:
    Given I send a "POST" request to the "stripe" service at "/test/reset"
    And I send a "POST" request to the "moesif" service at "/test/reset"

  Scenario: Setting up billing connects Moesif to Stripe and registers both event endpoints
    When I set up billing in the API Portal
    Then the response status code should be 201
    And the JSON response field "provider" should be "MOESIF"
    And the JSON response field "paymentProvider" should be "STRIPE"
    And the JSON response field "mode" should be "TEST"
    And the JSON response field "webhookEndpointUrl" should contain "/api-portal/billing/stripe/webhooks/"
    When I send a "GET" request to the "moesif" service at "/test/state"
    Then the response status should be 200
    And the JSON response field "partnerAccounts.stripe.partner_account.credentials.state" should be "enabled"
    When I send a "GET" request to the "stripe" service at "/test/state"
    Then the response status should be 200
    And the JSON response array field "webhookEndpoints" should have 2 items
    And the JSON response array "webhookEndpoints" should contain an item with "metadata.wso2_api_portal" equal to "true"
    And the JSON response array "webhookEndpoints" should contain an item with "description" equal to "Moesif: usage-based billing (added by the WSO2 API Portal)"

    # Billing is set up once.
    When I set up billing in the API Portal
    Then the response status code should be 409
    When I send an authenticated API Portal "GET" request to "/billing-configuration" as "admin"
    Then the response status code should be 200
    And the JSON response field "mode" should be "TEST"

    # Removing billing removes Moesif's connection and both endpoints.
    When I send an authenticated API Portal "DELETE" request to "/billing-configuration" as "admin"
    Then the response status code should be 204
    When I send a "GET" request to the "stripe" service at "/test/state"
    Then the JSON response array field "webhookEndpoints" should have 0 items
    When I send a "GET" request to the "moesif" service at "/test/state"
    Then the JSON response field "partnerAccounts.stripe" should not exist
    When I send an authenticated API Portal "GET" request to "/billing-configuration" as "admin"
    Then the response status code should be 404

  @type:negative
  Scenario: A Stripe key Stripe refuses is reported against it, and nothing is set up
    Given I send a "POST" request to the "stripe" service at "/test/fail" with body:
      """
      {"method":"GET","path":"/v1/webhook_endpoints","status":401}
      """
    When I set up billing in the API Portal
    Then the response status code should be 400
    And the JSON response field "code" should be "BILLING_KEY_INVALID"
    And the JSON response field "errors[0].field" should be "stripeApiKey"
    And the JSON response field "errors[0].message" should be "This Stripe key is invalid. Please enter a valid key."
    When I send a "GET" request to the "moesif" service at "/test/state"
    Then the JSON response field "partnerAccounts.stripe" should not exist
    When I send an authenticated API Portal "GET" request to "/billing-configuration" as "admin"
    Then the response status code should be 404

  @type:negative
  Scenario: A Moesif that can't be reached is reported as unavailable
    Given I send a "POST" request to the "moesif" service at "/test/fail" with body:
      """
      {"method":"GET","path":"/v1/~/billing/catalog/plans","status":503}
      """
    When I set up billing in the API Portal
    Then the response status code should be 502
    And the JSON response field "code" should be "BILLING_PROVIDER_UNAVAILABLE"
    And the JSON response field "errors[0].field" should be "moesifManagementKey"
    When I send an authenticated API Portal "GET" request to "/billing-configuration" as "admin"
    Then the response status code should be 404

  @type:negative
  Scenario: A step that fails undoes the steps before it
    Given I send a "POST" request to the "stripe" service at "/test/fail" with body:
      """
      {"method":"POST","path":"/v1/webhook_endpoints","match":"wso2_api_portal","status":400}
      """
    When I set up billing in the API Portal
    Then the response status code should be 400
    And the JSON response field "code" should be "BILLING_SETUP_FAILED"
    # Moesif's connection and Moesif's endpoint, made before the step that failed, are removed.
    When I send a "GET" request to the "moesif" service at "/test/state"
    Then the JSON response field "partnerAccounts.stripe" should not exist
    When I send a "GET" request to the "stripe" service at "/test/state"
    Then the JSON response array field "webhookEndpoints" should have 0 items
    When I send an authenticated API Portal "GET" request to "/billing-configuration" as "admin"
    Then the response status code should be 404

  @type:negative
  Scenario Outline: Keys that are not what billing needs are refused before any provider is called
    When I set up billing in the API Portal with values:
      | <field> | <value> |
    Then the response status code should be 400
    And the response body should contain "<message>"
    When I send a "GET" request to the "stripe" service at "/test/state"
    Then the JSON response array field "requests" should have 0 items

    Examples:
      | field                        | value                  | message                                                          |
      | stripeApiKey                 | pk_test_notasecretkey  | stripeApiKey is not in the expected format                       |
      | stripePublishableKey         | pk_live_livekey        | This is a live publishable key, but the secret key is a test key |
      | stripePublishableKey         | sk_test_secretkey      | stripePublishableKey is not in the expected format               |
      | moesifCollectorApplicationId | not-a-collector-id     | This is not a Collector Application Id                           |
      | webhookBaseUrl               | http://api-portal:9543 | Enter an https address                                           |
      | stripeApiKey                 |                        | stripeApiKey is required                                         |

  Scenario: The portal's event endpoint moves to another address, keeping its signing secret
    Given I set up billing in the API Portal
    And the response status code should be 201
    When I send an authenticated API Portal "PUT" request to "/billing-configuration/webhook-endpoint" as "admin" with JSON body:
      """
      {"webhookBaseUrl":"http://api-portal:9543"}
      """
    Then the response status code should be 400
    And the JSON response field "errors[0].field" should be "webhookBaseUrl"
    When I send an authenticated API Portal "PUT" request to "/billing-configuration/webhook-endpoint" as "admin" with JSON body:
      """
      {"webhookBaseUrl":"https://api-portal:9543/moved"}
      """
    Then the response status code should be 200
    And the JSON response field "webhookBaseUrl" should be "https://api-portal:9543/moved"
    And I store the JSON response field "webhookEndpointUrl" as "movedEndpointUrl"
    # Stripe's endpoint is moved, not replaced by a new one.
    When I send a "GET" request to the "stripe" service at "/test/state"
    Then the JSON response array field "webhookEndpoints" should have 2 items
    And the JSON response array "webhookEndpoints" item with "metadata.wso2_api_portal" equal to "true" should have "url" equal to "${CTX:movedEndpointUrl}"
