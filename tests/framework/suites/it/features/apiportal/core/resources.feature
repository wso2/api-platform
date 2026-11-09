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

@APIPORTAL @apiportal-resources
Feature: Applications, labels, views and key managers are managed through the API Portal REST API
  As an API Portal administrator or developer
  I want to manage the organization resources my role owns, and nothing more
  So that developers can obtain credentials while only administrators shape the portal

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml), IDs APIP-CORE-024..031, at the API level.
  # Developers hold read scopes on labels and views and none on key managers, so every write they
  # attempt there is refused and followed by a read that shows nothing was stored. Key managers
  # point at the runner's partition of the testbench OAuth2 mock; "test-secret" is that mock's fixed
  # client secret, never a real credential, and the portal must never echo or store it.

  @APIP-CORE-024
  Scenario: A developer manages an application from creation to deletion
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

  @APIP-CORE-025 @type:negative
  Scenario: An application without its required fields is refused and stores nothing
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

  @APIP-CORE-026
  Scenario: An administrator manages a label from creation to deletion
    Given I generate a unique resource name from "portal-label" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Premium APIs   |
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:labelId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Premium APIs"
    When I send an authenticated API Portal "GET" request to "/labels" as "admin"
    Then the response status code should be 200
    And the response body should contain "${CTX:labelId}"
    When I send an authenticated API Portal "PUT" request to "/labels/${CTX:labelId}" as "admin" with JSON values:
      | id          | ${CTX:labelId} |
      | displayName | Partner APIs   |
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Partner APIs"
    When I send an authenticated API Portal "DELETE" request to "/labels/${CTX:labelId}" as "admin"
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:labelId}" as "admin"
    Then the response status code should be 404

  @APIP-CORE-027
  Scenario: An administrator creates views with and without labels
    Given I generate a unique resource name from "portal-view-label" and store it as "labelId"
    And I generate a unique resource name from "portal-labelled-view" and store it as "labelledViewId"
    And I generate a unique resource name from "portal-unlabelled-view" and store it as "unlabelledViewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Partner APIs   |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "labelledViewId":
      | id          | ${CTX:labelledViewId} |
      | displayName | Partner View          |
      | labels      | ["${CTX:labelId}"]    |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "unlabelledViewId":
      | id          | ${CTX:unlabelledViewId} |
      | displayName | Empty View              |
    When I send an authenticated API Portal "GET" request to "/views/${CTX:labelledViewId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Partner View"
    And the JSON response array field "labels" should have 1 item
    And the JSON response field "labels[0]" should be "${CTX:labelId}"
    When I send an authenticated API Portal "GET" request to "/views/${CTX:unlabelledViewId}" as "admin"
    Then the response status code should be 200
    And the JSON response array field "labels" should have 0 items

  @APIP-CORE-028 @type:security
  Scenario: A developer can read labels and views but cannot create them
    Given I generate a unique resource name from "portal-readable-label" and store it as "labelId"
    And I generate a unique resource name from "portal-readable-view" and store it as "viewId"
    And I generate a unique resource name from "portal-denied-label" and store it as "deniedLabelId"
    And I generate a unique resource name from "portal-denied-view" and store it as "deniedViewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Public APIs    |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId} |
      | displayName | Public View   |
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:labelId}" as "developer"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}" as "developer"
    Then the response status code should be 200
    When I send an authenticated API Portal "POST" request to "/labels" as "developer" with JSON values:
      | id          | ${CTX:deniedLabelId} |
      | displayName | Denied Label         |
    Then the response status code should be 403
    When I send an authenticated API Portal "POST" request to "/views" as "developer" with JSON values:
      | id          | ${CTX:deniedViewId} |
      | displayName | Denied View         |
    Then the response status code should be 403
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:deniedLabelId}" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/views/${CTX:deniedViewId}" as "admin"
    Then the response status code should be 404

  @APIP-CORE-029
  Scenario: An administrator manages a key manager from creation to deletion
    Given I generate a unique resource name from "portal-km" and store it as "kmId"
    And a unique API Portal resource is created at "/key-managers" as "admin" with values and stored as "kmId":
      | id            | ${CTX:kmId}                                                          |
      | displayName   | Orders Key Manager                                                   |
      | tokenEndpoint | http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token |
    When I send an authenticated API Portal "GET" request to "/key-managers/${CTX:kmId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Orders Key Manager"
    When I send an authenticated API Portal "GET" request to "/key-managers" as "admin"
    Then the response status code should be 200
    And the response body should contain "${CTX:kmId}"
    When I send an authenticated API Portal "PUT" request to "/key-managers/${CTX:kmId}" as "admin" with JSON values:
      | displayName | Renamed Key Manager |
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Renamed Key Manager"
    When I send an authenticated API Portal "DELETE" request to "/key-managers/${CTX:kmId}" as "admin"
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/key-managers/${CTX:kmId}" as "admin"
    Then the response status code should be 404

  @APIP-CORE-030 @type:security
  Scenario: A developer gets a token through an application's key-manager mapping, and only with the right secret
    Given I generate a unique resource name from "portal-token-km" and store it as "kmId"
    And I generate a unique resource name from "portal-token-app" and store it as "appId"
    And a unique API Portal resource is created at "/key-managers" as "admin" with values and stored as "kmId":
      | id            | ${CTX:kmId}                                                                    |
      | displayName   | Token Key Manager                                                              |
      | tokenEndpoint | http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token?ttl=3600 |
    And a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}           |
      | displayName | Token Application      |
      | description | Requests access tokens |
    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/generate-keys" as "developer" with JSON values:
      | keyManager  | ${CTX:kmId} |
      | type        | PRODUCTION  |
      | consumerKey | test-client |
    Then the response status code should be 200
    And I store the JSON response field "keyMappingId" as "keyMappingId"
    And I register API Portal application key mapping "${CTX:keyMappingId}" for application "${CTX:appId}" for cleanup

    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/oauth-keys/${CTX:keyMappingId}/generate-token" as "developer" with JSON values:
      | consumerSecret | wrong-secret |
    Then the response should be a client error
    And the response body should not contain "wrong-secret"

    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/oauth-keys/${CTX:keyMappingId}/generate-token" as "developer" with JSON values:
      | consumerSecret | test-secret |
    Then the response status code should be 200
    And the response body should contain "mock-token"
    And the JSON response field "validityTime" should be 3600
    And the JSON response array field "tokenScopes" should have 1 item
    And the JSON response field "tokenScopes[0]" should be "default"
    And the response body should not contain "test-secret"
    When I send an authenticated API Portal "GET" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 200
    And the response body should not contain "test-secret"

  @APIP-CORE-031 @type:security
  Scenario: A developer cannot register a key manager
    Given I generate a unique resource name from "portal-denied-km" and store it as "deniedKmId"
    And I generate a unique resource name from "portal-allowed-km" and store it as "kmId"
    When I send an authenticated API Portal "POST" request to "/key-managers" as "developer" with JSON values:
      | id            | ${CTX:deniedKmId}                                                    |
      | displayName   | Denied Key Manager                                                   |
      | tokenEndpoint | http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token |
    Then the response status code should be 403
    When I send an authenticated API Portal "GET" request to "/key-managers/${CTX:deniedKmId}" as "admin"
    Then the response status code should be 404
    And a unique API Portal resource is created at "/key-managers" as "admin" with values and stored as "kmId":
      | id            | ${CTX:kmId}                                                          |
      | displayName   | Allowed Key Manager                                                  |
      | tokenEndpoint | http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token |
    When I send an authenticated API Portal "GET" request to "/key-managers/${CTX:kmId}" as "admin"
    Then the response status code should be 200
