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

Feature: API Portal webhook delivery pipeline

  Scenario: A disabled subscriber receives no application event
    Given I generate a unique resource name from "portal-disabled-delivery-subscriber" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"Disabled Delivery Subscriber","targetUrl":"http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook","events":["application.*"],"enabled":false}
      """
    And I generate a unique resource name from "portal-disabled-delivery-app" and store it as "appId"
    When a unique API Portal resource is created at "/applications" as "developer" with body and stored as "appId":
      """
      {"id":"${CTX:appId}","displayName":"Disabled Delivery Application","description":"disabled"}
      """
    Then the response status code should be 201
    And I verify no API Portal webhook event "application.created" containing "${CTX:appId}" is delivered

  Scenario: A non-successful webhook response is recorded as a failed delivery
    Given I generate a unique resource name from "portal-failed-delivery-subscriber" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"Failed Delivery Subscriber","targetUrl":"http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook?status=500","events":["application.*"]}
      """
    And I generate a unique resource name from "portal-failed-delivery-app" and store it as "appId"
    And I reset API Portal webhook events
    When a unique API Portal resource is created at "/applications" as "developer" with body and stored as "appId":
      """
      {"id":"${CTX:appId}","displayName":"Failed Delivery Application","description":"failed"}
      """
    Then the response status code should be 201
    When I wait for an API Portal webhook event "application.created" containing "${CTX:appId}"
    Then the JSON response field "event_type" should be "application.created"
    And I store the JSON response field "event_id" as "eventId"
    And I wait for API Portal webhook event "${CTX:eventId}" to reach status "FAILED"
    When I send an authenticated API Portal "GET" request to "/webhook-subscribers/${CTX:subscriberId}/deliveries" as "admin"
    Then the response status code should be 200
    And the response body should contain "FAILED"
    And the response body should contain "500"

  Scenario: A successfully delivered event reaches ALL_DELIVERED status
    Given I generate a unique resource name from "portal-all-delivered-subscriber" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "subscriberId":
      | id          | ${CTX:subscriberId}                              |
      | displayName | All Delivered Subscriber                         |
      | targetUrl   | http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook   |
      | events      | ["application.*"]                              |
    And I generate a unique resource name from "portal-all-delivered-app" and store it as "appId"
    And I reset API Portal webhook events
    When a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}                   |
      | displayName | All Delivered Application    |
      | description | all-delivered               |
    Then the response status code should be 201
    When I wait for an API Portal webhook event "application.created" containing "${CTX:appId}"
    Then I store the JSON response field "event_id" as "eventId"
    Then I wait for API Portal webhook event "${CTX:eventId}" to reach status "ALL_DELIVERED"

  Scenario: A subscriber secret signs the raw webhook payload
    Given I generate a unique resource name from "portal-signing-secret" and store it as "signingSecret"
    And I generate a unique resource name from "portal-signed-delivery-subscriber" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "subscriberId":
      | id          | ${CTX:subscriberId}                                         |
      | displayName | Signed Delivery Subscriber                                |
      | targetUrl   | http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook            |
      | events      | ["application.*"]                                         |
      | secret      | ${CTX:signingSecret}                                       |
    And I generate a unique resource name from "portal-signed-delivery-app" and store it as "appId"
    And I reset API Portal webhook events
    When a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}                  |
      | displayName | Signed Delivery Application |
      | description | signed                      |
    Then the response status code should be 201
    And the API Portal webhook delivery "application.created" containing "${CTX:appId}" should have a valid signature using secret "${CTX:signingSecret}"
