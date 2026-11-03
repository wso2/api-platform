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

Feature: API Portal application webhook events

  Scenario: Application creation is delivered to a matching subscriber
    Given I generate a unique resource name from "portal-application-webhook" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"Application Events","targetUrl":"http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook","events":["application.*"]}
      """
    And I generate a unique resource name from "portal-webhook-app" and store it as "appId"
    And a unique API Portal resource is created at "/applications" as "developer" with body and stored as "appId":
      """
      {"id":"${CTX:appId}","displayName":"Webhook Application","description":"created"}
      """
    When I wait for an API Portal webhook event "application.created" containing "${CTX:appId}"
    Then the JSON response field "event_type" should be "application.created"
    And the JSON response field "data.handle" should be "${CTX:appId}"

  Scenario: Application updates are delivered to a matching subscriber
    Given I generate a unique resource name from "portal-application-update-webhook" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"Application Update Events","targetUrl":"http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook","events":["application.*"]}
      """
    And I generate a unique resource name from "portal-update-webhook-app" and store it as "appId"
    And a unique API Portal resource is created at "/applications" as "developer" with body and stored as "appId":
      """
      {"id":"${CTX:appId}","displayName":"Original Webhook Application","description":"before"}
      """
    When I send an authenticated API Portal "PUT" request to "/applications/${CTX:appId}" as "developer" with JSON body:
      """
      {"displayName":"Updated Webhook Application","description":"after"}
      """
    Then the response status code should be 200
    When I wait for an API Portal webhook event "application.updated" containing "${CTX:appId}"
    Then the JSON response field "event_type" should be "application.updated"
    And the JSON response field "data.display_name" should be "Updated Webhook Application"

  Scenario: Application deletion is delivered to a matching subscriber
    Given I generate a unique resource name from "portal-application-delete-webhook" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"Application Delete Events","targetUrl":"http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook","events":["application.*"]}
      """
    And I generate a unique resource name from "portal-delete-webhook-app" and store it as "appId"
    And a unique API Portal resource is created at "/applications" as "developer" with body and stored as "appId":
      """
      {"id":"${CTX:appId}","displayName":"Delete Webhook Application","description":"delete"}
      """
    When I send an authenticated API Portal "DELETE" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 200
    When I wait for an API Portal webhook event "application.deleted" containing "${CTX:appId}"
    Then the JSON response field "event_type" should be "application.deleted"

  Scenario: An application event is not delivered to a non-matching subscriber
    Given I generate a unique resource name from "portal-application-pattern-webhook" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"API Key Events Only","targetUrl":"http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook","events":["apikey.*"]}
      """
    And I generate a unique resource name from "portal-pattern-webhook-app" and store it as "appId"
    When a unique API Portal resource is created at "/applications" as "developer" with body and stored as "appId":
      """
      {"id":"${CTX:appId}","displayName":"Pattern Mismatch Application","description":"not delivered"}
      """
    Then the response status code should be 201
    And I verify no API Portal webhook event "application.created" containing "${CTX:appId}" is delivered

  Scenario: Deleting an application twice does not publish a second event
    Given I generate a unique resource name from "portal-application-delete-once-subscriber" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"Application Delete Once Events","targetUrl":"http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook","events":["application.*"]}
      """
    And I generate a unique resource name from "portal-application-delete-once-webhook" and store it as "appId"
    And a unique API Portal resource is created at "/applications" as "developer" with body and stored as "appId":
      """
      {"id":"${CTX:appId}","displayName":"Delete Once Application","description":"delete once"}
      """
    When I send an authenticated API Portal "DELETE" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 200
    When I wait for an API Portal webhook event "application.deleted" containing "${CTX:appId}"
    And I reset API Portal webhook events
    When I send an authenticated API Portal "DELETE" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 404
    And I verify no API Portal webhook event "application.deleted" containing "${CTX:appId}" is delivered
