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

@APIPORTAL @apiportal-applications
Feature: Developers own the applications they create in the API Portal
  As an API Portal developer
  I want to create, rename and delete applications that only I can see
  So that the credentials I group in them stay mine and are named the way I expect

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml) at the API level.
  # The application events are asserted by webhooks/api_key_webhooks.feature,
  # and deleting an application that holds credentials by applications/application_deletion.feature.
  # The suite has one developer, so the administrator, who holds every
  # application scope, plays the second user: applications are owned by their creator, not by role.

  @APIP-014 @type:boundary @type:negative @type:security
  Scenario: A developer manages applications within their limits and sees only their own
    # A developer manages an application from creation to deletion
    Given I generate a unique resource name from "portal-app" and store it as "appId"
    And a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}           |
      | displayName | Orders Application     |
      | description | Calls the orders APIs  |
    When I send an authenticated API Portal "GET" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Orders Application"
    When I send an authenticated API Portal "GET" request to "/applications" as "developer"
    Then the response status code should be 200
    And the response body should contain "${CTX:appId}"
    When I send an authenticated API Portal "PUT" request to "/applications/${CTX:appId}" as "developer" with JSON values:
      | displayName | Renamed Orders Application |
      | description | Renamed                    |
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Renamed Orders Application"
    When I send an authenticated API Portal "DELETE" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 404

    # A developer creates an application without a handle, and every later call uses the handle derived from its name
    Given I generate a unique resource name from "portal-app" and store it as "appSuffix"
    When a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | displayName | Orders_App Name ${CTX:appSuffix} |
      | description | Calls the orders APIs            |
    Then the response status code should be 201
    And the JSON response field "id" should be "orders-app-name-${CTX:appSuffix}"
    And the JSON response field "displayName" should be "Orders_App Name ${CTX:appSuffix}"

    When I send an authenticated API Portal "GET" request to "/applications/orders-app-name-${CTX:appSuffix}" as "developer"
    Then the response status code should be 200
    And the JSON response field "description" should be "Calls the orders APIs"
    When I send an authenticated API Portal "GET" request to "/applications" as "developer"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:appId}"

    When I send an authenticated API Portal "PUT" request to "/applications/${CTX:appId}" as "developer" with JSON values:
      | displayName | Renamed Orders App |
      | description | Renamed            |
    Then the response status code should be 200
    And the JSON response field "id" should be "${CTX:appId}"
    And the JSON response field "displayName" should be "Renamed Orders App"
    When I send an authenticated API Portal "GET" request to "/applications/${CTX:appId}" as "developer"
    Then the JSON response field "displayName" should be "Renamed Orders App"
    And the JSON response field "description" should be "Renamed"

    When I send an authenticated API Portal "DELETE" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/applications" as "developer"
    Then the JSON response array "list" should not contain an item with "id" equal to "${CTX:appId}"

    # An application with a 100-character name is accepted and stored as sent
    Given I generate a unique resource name from "portal-app-boundary" and store it as "appId"
    When a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}    |
      | displayName | Orders application xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx   |
      | description | Longest allowed name   |
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Orders application xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
    And the JSON response field "description" should be "Longest allowed name"

    # An application with a 256-character description is accepted and stored as sent
    Given I generate a unique resource name from "portal-app-boundary" and store it as "appId"
    When a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}    |
      | displayName | Orders application   |
      | description | Calls the orders APIs xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx   |
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Orders application"
    And the JSON response field "description" should be "Calls the orders APIs xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

    # An application without its required fields is refused and stores nothing
    Given I generate a unique resource name from "portal-app-invalid" and store it as "appId"
    When I send an authenticated API Portal "POST" request to "/applications" as "developer" with JSON values:
      | id | ${CTX:appId} |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 404
    And a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}          |
      | displayName | Orders Application    |
      | description | Calls the orders APIs |

    # An application without a name, or with a handle already in use, is refused and the first one is unchanged
    Given I generate a unique resource name from "portal-app-taken" and store it as "appId"
    And I generate a unique resource name from "portal-app-unnamed" and store it as "unnamedAppId"
    And I generate a unique resource name from "portal-app-name" and store it as "nameSuffix"
    And a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}          |
      | displayName | Orders Application    |
      | description | Calls the orders APIs |
    When I send an authenticated API Portal "POST" request to "/applications" as "developer" with JSON values:
      | id          | ${CTX:appId}       |
      | displayName | Second Application |
      | description | Takes a used id    |
    Then the response status code should be 409
    And the response body should not contain "stack"
    When I send an authenticated API Portal "POST" request to "/applications" as "developer" with JSON values:
      | id          | ${CTX:unnamedAppId} |
      | description | Has no name         |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/applications/${CTX:unnamedAppId}" as "developer"
    Then the response status code should be 404

    # Two applications named alike without an id derive the same handle.
    When a unique API Portal resource is created at "/applications" as "developer" with values and stored as "namedAppId":
      | displayName | Shared Name ${CTX:nameSuffix} |
      | description | First of the name             |
    Then the JSON response field "id" should be "shared-name-${CTX:nameSuffix}"
    When I send an authenticated API Portal "POST" request to "/applications" as "developer" with JSON values:
      | displayName | Shared Name ${CTX:nameSuffix} |
      | description | Second of the name            |
    Then the response status code should be 409

    When I send an authenticated API Portal "GET" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Orders Application"
    When I send an authenticated API Portal "GET" request to "/applications/${CTX:namedAppId}" as "developer"
    Then the JSON response field "description" should be "First of the name"

    # An application lists the API keys linked to it, and only those
    Given a REST API is created in the API Portal and stored as "apiId"
    And I generate a unique resource name from "portal-app-keys" and store it as "appId"
    And a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}      |
      | displayName | Key Application   |
      | description | Holds linked keys |
    And a "developer" API key for API "${CTX:apiId}" is generated and stored as "firstKeyId"
    And a "developer" API key for API "${CTX:apiId}" is generated and stored as "secondKeyId"
    And a "developer" API key for API "${CTX:apiId}" is generated and stored as "unlinkedKeyId"
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/associate" as "developer" with JSON values:
      | keyId | ${CTX:firstKeyId} |
      | appId | ${CTX:appId}      |
    Then the response status code should be 200
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/associate" as "developer" with JSON values:
      | keyId | ${CTX:secondKeyId} |
      | appId | ${CTX:appId}       |
    Then the response status code should be 200

    When I send an authenticated API Portal "GET" request to "/applications/${CTX:appId}/api-keys" as "developer"
    Then the response status code should be 200
    And the JSON response array field "list" should have 2 items
    And the JSON response array "list" item with "id" equal to "${CTX:firstKeyId}" should have "apiId" equal to "${CTX:apiId}"
    And the JSON response array "list" item with "id" equal to "${CTX:firstKeyId}" should have "appId" equal to "${CTX:appId}"
    And the JSON response array "list" item with "id" equal to "${CTX:secondKeyId}" should have "apiId" equal to "${CTX:apiId}"
    And the JSON response array "list" item with "id" equal to "${CTX:secondKeyId}" should have "status" equal to "ACTIVE"
    And the JSON response array "list" should not contain an item with "id" equal to "${CTX:unlinkedKeyId}"
    And the JSON response array "list" item with "id" equal to "${CTX:firstKeyId}" should not have field "key"

    # Another user can neither see nor change a developer's application
    Given I generate a unique resource name from "portal-app-owned" and store it as "appId"
    And a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}          |
      | displayName | Orders Application    |
      | description | Calls the orders APIs |
    When I send an authenticated API Portal "GET" request to "/applications" as "admin"
    Then the response status code should be 200
    And the JSON response array "list" should not contain an item with "id" equal to "${CTX:appId}"
    When I send an authenticated API Portal "GET" request to "/applications/${CTX:appId}" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "PUT" request to "/applications/${CTX:appId}" as "admin" with JSON values:
      | displayName | Taken Over |
      | description | Taken Over |
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/applications/${CTX:appId}/api-keys" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/applications/${CTX:appId}/oauth2-keys" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "DELETE" request to "/applications/${CTX:appId}" as "admin"
    Then the response status code should be 404
    And the response body should not contain "stack"

    When I send an authenticated API Portal "GET" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Orders Application"
    And the JSON response field "description" should be "Calls the orders APIs"
    When I send an authenticated API Portal "GET" request to "/applications" as "developer"
    Then the JSON response array "list" should contain an item with "id" equal to "${CTX:appId}"
