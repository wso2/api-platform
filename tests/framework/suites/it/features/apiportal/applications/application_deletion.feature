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


@APIPORTAL @apiportal-application-deletion
Feature: Deleting an application unlinks its credentials without revoking them
  As an API Portal developer
  I want deleting an application to remove only the application
  So that the API keys, subscriptions and OAuth2 clients I grouped in it keep working

  # Scenario APIPORTAL (scenarios/APIPORTAL.yaml) at the API level. It runs
  # in the api-portal-webhooks block because webhook subscribers are organization-wide. The key
  # manager points at this scenario's partition of the testbench OAuth2 mock, whose fixed client is
  # "test-client" / "test-secret" — a mock credential, never a real one.

  @APIP-014
  Scenario: Deleting an application unlinks its API keys, which stay active, and leaves its subscription and key manager alone
    # Deleting an application unlinks its API keys, which stay active, and leaves its subscription and key manager alone
    Given a REST API is created in the API Portal and stored as "apiId"
    And I generate a unique resource name from "portal-deleted-app-subscriber" and store it as "subscriberId"
    And I generate a unique resource name from "portal-deleted-app-km" and store it as "kmId"
    And I generate a unique resource name from "portal-deleted-app" and store it as "appId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "subscriberId":
      | id          | ${CTX:subscriberId} |
      | displayName | ${CTX:subscriberId} |
      | targetUrl   | http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook |
      | events      | ["apikey.*"] |
    And a unique API Portal resource is created at "/key-managers" as "admin" with values and stored as "kmId":
      | id            | ${CTX:kmId}                                                          |
      | displayName   | Deleted App Key Manager                                              |
      | tokenEndpoint | http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token |
    And a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}          |
      | displayName | Short-lived App       |
      | description | Deleted with its keys |
    And a "developer" API key for API "${CTX:apiId}" is generated and stored as "firstKeyId"
    And a "developer" API key for API "${CTX:apiId}" is generated and stored as "secondKeyId"
    And a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/associate" as "developer" with JSON values:
      | keyId | ${CTX:firstKeyId} |
      | appId | ${CTX:appId}      |
    Then the response status code should be 200
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/associate" as "developer" with JSON values:
      | keyId | ${CTX:secondKeyId} |
      | appId | ${CTX:appId}       |
    Then the response status code should be 200
    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/generate-keys" as "developer" with JSON values:
      | keyManager  | ${CTX:kmId} |
      | type        | PRODUCTION  |
      | consumerKey | test-client |
    Then the response status code should be 200
    And I store the JSON response field "keyMappingId" as "keyMappingId"
    And I register API Portal application key mapping "${CTX:keyMappingId}" for application "${CTX:appId}" for cleanup
    When I send an authenticated API Portal "GET" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 200
    # Linking each key was reported with the application; both deliveries land before the reset,
    # so only what the deletion sends is read below.
    When I wait for an API Portal webhook event "apikey.application_updated" containing "${CTX:firstKeyId}"
    Then the JSON response field "data.application.handle" should be "${CTX:appId}"
    When I wait for an API Portal webhook event "apikey.application_updated" containing "${CTX:secondKeyId}"
    Then the JSON response field "data.application.handle" should be "${CTX:appId}"

    And I reset API Portal webhook events
    When I send an authenticated API Portal "DELETE" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 200
    And I deregister API Portal resource "${CTX:appId}" at "/applications" from cleanup

    # Each key is unlinked once, and reported with no application.
    When I wait for an API Portal webhook event "apikey.application_updated" containing "${CTX:firstKeyId}" and not containing "${CTX:appId}"
    Then the JSON response field "data.handle" should be "${CTX:firstKeyId}"
    And the JSON response field "data.application" should be null
    When I wait for an API Portal webhook event "apikey.application_updated" containing "${CTX:secondKeyId}" and not containing "${CTX:appId}"
    Then the JSON response field "data.handle" should be "${CTX:secondKeyId}"
    And the JSON response field "data.application" should be null
    And the API Portal webhook deliveries containing "${CTX:firstKeyId}" should be 1, each carrying its event headers and envelope
    And the API Portal webhook deliveries containing "${CTX:secondKeyId}" should be 1, each carrying its event headers and envelope

    # The keys stay active and unlinked.
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/api-keys" as "developer"
    Then the response status code should be 200
    And the JSON response array "list" item with "id" equal to "${CTX:firstKeyId}" should have "status" equal to "ACTIVE"
    And the JSON response array "list" item with "id" equal to "${CTX:firstKeyId}" should have "appId" equal to "null"
    And the JSON response array "list" item with "id" equal to "${CTX:secondKeyId}" should have "status" equal to "ACTIVE"
    And the JSON response array "list" item with "id" equal to "${CTX:secondKeyId}" should have "appId" equal to "null"

    # The subscription and the key manager are untouched.
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the response status code should be 200
    And the JSON response field "subscriptionPlanName" should be "Gold"
    When I send an authenticated API Portal "GET" request to "/key-managers/${CTX:kmId}" as "admin"
    Then the response status code should be 200

    # The client mapping went with the application: an application created again under the same
    # handle starts with none, and the old mapping issues no token.
    When a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}        |
      | displayName | Recreated App       |
      | description | Same handle, no keys |
    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/oauth-keys/${CTX:keyMappingId}/generate-token" as "developer" with JSON values:
      | consumerSecret | test-secret |
    Then the response status code should be 404
    And the response body should not contain "test-secret"
