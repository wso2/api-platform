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

Feature: API Portal webhook subscribers

  Scenario: An administrator creates and retrieves a webhook subscriber
    Given I generate a unique resource name from "portal-webhook" and store it as "subscriberId"
    And I generate a unique resource name from "portal-webhook-secret" and store it as "secret"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"Webhook Subscriber","targetUrl":"https://example.invalid/webhook","secret":"${CTX:secret}","events":["apikey.*"]}
      """
    When I send an authenticated API Portal "GET" request to "/webhook-subscribers/${CTX:subscriberId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "targetUrl" should be "https://example.invalid/webhook"
    And the JSON response field "secret" should not exist

  Scenario: An administrator receives a generated webhook subscriber handle
    Given I generate a unique resource name from "portal-generated-webhook" and store it as "displayName"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"displayName":"Generated ${CTX:displayName}","targetUrl":"https://example.invalid/webhook","events":["apikey.*"]}
      """
    When I send an authenticated API Portal "GET" request to "/webhook-subscribers/${CTX:subscriberId}" as "admin"
    Then the response status code should be 200
    And the response body should contain "Generated ${CTX:displayName}"

  Scenario: Webhook subscribers with the same display name receive distinct handles
    Given I generate a unique resource name from "portal-duplicate-webhook" and store it as "displayName"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "firstSubscriberId":
      """
      {"displayName":"Duplicate ${CTX:displayName}","targetUrl":"https://example.invalid/webhook"}
      """
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "secondSubscriberId":
      """
      {"displayName":"Duplicate ${CTX:displayName}","targetUrl":"https://example.invalid/webhook"}
      """
    When I send an authenticated API Portal "GET" request to "/webhook-subscribers/${CTX:secondSubscriberId}" as "admin"
    Then the response status code should be 200
    And the response body should contain "Duplicate ${CTX:displayName}"

  Scenario: An administrator updates a webhook subscriber
    Given I generate a unique resource name from "portal-update-webhook" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"Update Subscriber","targetUrl":"https://example.invalid/webhook","events":["apikey.*"]}
      """
    When I send an authenticated API Portal "PUT" request to "/webhook-subscribers/${CTX:subscriberId}" as "admin" with JSON body:
      """
      {"displayName":"Update Subscriber","targetUrl":"https://updated.example.invalid/webhook","events":["subscription.*"],"enabled":false}
      """
    Then the response status code should be 200
    And the JSON response field "targetUrl" should be "https://updated.example.invalid/webhook"
    And the JSON response field "enabled" should be "false"

  Scenario: An administrator deletes a webhook subscriber
    Given I generate a unique resource name from "portal-delete-webhook" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"Delete Subscriber","targetUrl":"https://example.invalid/webhook"}
      """
    When I send an authenticated API Portal "DELETE" request to "/webhook-subscribers/${CTX:subscriberId}" as "admin"
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/webhook-subscribers/${CTX:subscriberId}" as "admin"
    Then the response status code should be 404

  Scenario: An administrator lists webhook deliveries for a subscriber
    Given I generate a unique resource name from "portal-delivery-webhook" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"Delivery Subscriber","targetUrl":"https://example.invalid/webhook","events":["application.*"]}
      """
    And I generate a unique resource name from "portal-delivery-app" and store it as "appId"
    And a unique API Portal resource is created at "/applications" as "developer" with body and stored as "appId":
      """
      {"id":"${CTX:appId}","displayName":"Delivery Application","description":"delivery"}
      """
    When I send an authenticated API Portal "GET" request to "/webhook-subscribers/${CTX:subscriberId}/deliveries" as "admin"
    Then the response status code should be 200
    And the API Portal delivery for subscriber "${CTX:subscriberId}" and event "application.created" should be "FAILED"

  Scenario: A webhook subscriber accepts an exact event pattern
    Given I generate a unique resource name from "portal-literal-webhook" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"Literal Subscriber","targetUrl":"https://example.invalid/webhook","events":["*.created"]}
      """
    When I send an authenticated API Portal "GET" request to "/webhook-subscribers/${CTX:subscriberId}" as "admin"
    Then the response status code should be 200
    And the response body should contain "*.created"
