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


@APIPORTAL @apiportal-oauth2-keys
Feature: Developers obtain OAuth2 credentials and tokens through the API Portal
  As an API Portal developer
  I want to link the OAuth2 clients I own to my applications and get tokens with them
  So that I can call OAuth2-secured APIs in production and sandbox, and the portal never keeps my secret

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml) at the API level.
  # Key managers point at this scenario's partition of the testbench OAuth2 mock, whose fixed client is
  # "test-client" / "test-secret" — a mock credential, never a real one. The portal must never echo
  # or store the secret. Importing a client uses a "provision" key manager, which registers nothing
  # at the identity provider; its provisioning password is generated per run.

  @APIP-016
  Scenario: An application maps production and sandbox clients and gets tokens through them
    # An application holds a production and a sandbox client, and each issues tokens
    Given I generate a unique resource name from "portal-prod-km" and store it as "prodKmId"
    And I generate a unique resource name from "portal-sandbox-km" and store it as "sandboxKmId"
    And I generate a unique resource name from "portal-env-app" and store it as "appId"
    And a unique API Portal resource is created at "/key-managers" as "admin" with values and stored as "prodKmId":
      | id            | ${CTX:prodKmId}                                                      |
      | displayName   | Production Key Manager                                               |
      | keyType       | PRODUCTION                                                           |
      | tokenEndpoint | http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token |
    And a unique API Portal resource is created at "/key-managers" as "admin" with values and stored as "sandboxKmId":
      | id            | ${CTX:sandboxKmId}                                                   |
      | displayName   | Sandbox Key Manager                                                  |
      | keyType       | SANDBOX                                                              |
      | tokenEndpoint | http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token |
    And a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}            |
      | displayName | Two Environments App    |
      | description | Production and sandbox  |
    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/generate-keys" as "developer" with JSON values:
      | keyManager  | ${CTX:prodKmId} |
      | type        | PRODUCTION      |
      | consumerKey | test-client     |
    Then the response status code should be 200
    And the JSON response field "type" should be "PRODUCTION"
    And I store the JSON response field "keyMappingId" as "prodMappingId"
    And I register API Portal application key mapping "${CTX:prodMappingId}" for application "${CTX:appId}" for cleanup
    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/generate-keys" as "developer" with JSON values:
      | keyManager  | ${CTX:sandboxKmId} |
      | type        | SANDBOX            |
      | consumerKey | test-client        |
    Then the response status code should be 200
    And the JSON response field "type" should be "SANDBOX"
    And I store the JSON response field "keyMappingId" as "sandboxMappingId"
    And I register API Portal application key mapping "${CTX:sandboxMappingId}" for application "${CTX:appId}" for cleanup

    When I send an authenticated API Portal "GET" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 200
    And the response body should not contain "test-secret"
    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/oauth-keys/${CTX:prodMappingId}/generate-token" as "developer" with JSON values:
      | consumerSecret | test-secret |
    Then the response status code should be 200
    And the response body should contain "mock-token"
    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/oauth-keys/${CTX:sandboxMappingId}/generate-token" as "developer" with JSON values:
      | consumerSecret | test-secret |
    Then the response status code should be 200
    And the response body should contain "mock-token"

    # A token is issued with the scopes the key manager granted
    Given I generate a unique resource name from "portal-scopes-km" and store it as "kmId"
    And I generate a unique resource name from "portal-scopes-app" and store it as "appId"
    And a unique API Portal resource is created at "/key-managers" as "admin" with values and stored as "kmId":
      | id            | ${CTX:kmId}                                                          |
      | displayName   | Scopes Key Manager                                                   |
      | tokenEndpoint | http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token |
    And a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}       |
      | displayName | Scopes Application |
      | description | Requests scopes    |
    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/generate-keys" as "developer" with JSON values:
      | keyManager  | ${CTX:kmId} |
      | type        | PRODUCTION  |
      | consumerKey | test-client |
    Then the response status code should be 200
    And I store the JSON response field "keyMappingId" as "keyMappingId"
    And I register API Portal application key mapping "${CTX:keyMappingId}" for application "${CTX:appId}" for cleanup
    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/oauth-keys/${CTX:keyMappingId}/generate-token" as "developer" with JSON values:
      | consumerSecret | test-secret              |
      | scopes         | ["read","write","admin"] |
    Then the response status code should be 200
    And the response body should contain "mock-token"
    And the JSON response array field "tokenScopes" should have 3 items
    And the JSON response field "tokenScopes[0]" should be "read"
    And the JSON response field "tokenScopes[1]" should be "write"
    And the JSON response field "tokenScopes[2]" should be "admin"
    And the response body should not contain "test-secret"

    # An application stays readable after production and sandbox clients are mapped to it
    Given I generate a unique resource name from "portal-listed-prod-km" and store it as "prodKmId"
    And I generate a unique resource name from "portal-listed-sandbox-km" and store it as "sandboxKmId"
    And I generate a unique resource name from "portal-listed-app" and store it as "appId"
    And a unique API Portal resource is created at "/key-managers" as "admin" with values and stored as "prodKmId":
      | id            | ${CTX:prodKmId}                                                      |
      | displayName   | Production Key Manager                                               |
      | keyType       | PRODUCTION                                                           |
      | tokenEndpoint | http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token |
    And a unique API Portal resource is created at "/key-managers" as "admin" with values and stored as "sandboxKmId":
      | id            | ${CTX:sandboxKmId}                                                   |
      | displayName   | Sandbox Key Manager                                                  |
      | keyType       | SANDBOX                                                              |
      | tokenEndpoint | http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token |
    And a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}           |
      | displayName | Listed Clients App     |
      | description | Production and sandbox |
    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/generate-keys" as "developer" with JSON values:
      | keyManager  | ${CTX:prodKmId} |
      | type        | PRODUCTION      |
      | consumerKey | test-client     |
    Then the response status code should be 200
    And I store the JSON response field "keyMappingId" as "prodMappingId"
    And I register API Portal application key mapping "${CTX:prodMappingId}" for application "${CTX:appId}" for cleanup
    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/generate-keys" as "developer" with JSON values:
      | keyManager  | ${CTX:sandboxKmId} |
      | type        | SANDBOX            |
      | consumerKey | test-client        |
    Then the response status code should be 200
    And I store the JSON response field "keyMappingId" as "sandboxMappingId"
    And I register API Portal application key mapping "${CTX:sandboxMappingId}" for application "${CTX:appId}" for cleanup
    When I send an authenticated API Portal "GET" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Listed Clients App"
    And the JSON response field "description" should be "Production and sandbox"

    # Revoking an application's client mapping removes it from the application and from token issuance
    Given I generate a unique resource name from "portal-revoke-km" and store it as "kmId"
    And I generate a unique resource name from "portal-revoke-app" and store it as "appId"
    And a unique API Portal resource is created at "/key-managers" as "admin" with values and stored as "kmId":
      | id            | ${CTX:kmId}                                                          |
      | displayName   | Revoke Key Manager                                                   |
      | tokenEndpoint | http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token |
    And a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}        |
      | displayName | Revoke Application  |
      | description | Drops its mapping   |
    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/generate-keys" as "developer" with JSON values:
      | keyManager  | ${CTX:kmId} |
      | type        | PRODUCTION  |
      | consumerKey | test-client |
    Then the response status code should be 200
    And I store the JSON response field "keyMappingId" as "keyMappingId"
    And I register API Portal application key mapping "${CTX:keyMappingId}" for application "${CTX:appId}" for cleanup
    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/oauth-keys/${CTX:keyMappingId}/generate-token" as "developer" with JSON values:
      | consumerSecret | test-secret |
    Then the response status code should be 200

    When I send an authenticated API Portal "DELETE" request to "/applications/${CTX:appId}/oauth-keys/${CTX:keyMappingId}" as "developer"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 200
    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/oauth-keys/${CTX:keyMappingId}/generate-token" as "developer" with JSON values:
      | consumerSecret | test-secret |
    Then the response status code should be 404
    And the response body should not contain "mock-token"
    When I send an authenticated API Portal "DELETE" request to "/applications/${CTX:appId}/oauth-keys/${CTX:keyMappingId}" as "developer"
    Then the response status code should be 404

    # The key manager itself is untouched, and the same client can be mapped again.
    When I send an authenticated API Portal "GET" request to "/key-managers/${CTX:kmId}" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/generate-keys" as "developer" with JSON values:
      | keyManager  | ${CTX:kmId} |
      | type        | PRODUCTION  |
      | consumerKey | test-client |
    Then the response status code should be 200
    And I store the JSON response field "keyMappingId" as "remappedMappingId"
    And I register API Portal application key mapping "${CTX:remappedMappingId}" for application "${CTX:appId}" for cleanup

  @APIP-016 @type:security
  Scenario: An OAuth2 key is imported, linked and removed by its owner only
    # An OAuth2 key imported for an existing client never shows a secret, is its owner's only, and is removed with its record
    Given I generate a unique resource name from "portal-import-km" and store it as "kmId"
    And I generate a unique value from "portal-km-password" and store it as "provisioningPassword"
    And a unique API Portal resource is created at "/key-managers" as "admin" with values and stored as "kmId":
      | id            | ${CTX:kmId}                                                          |
      | displayName   | Import Key Manager                                                   |
      | tokenEndpoint | http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token |
      | provisioning  | {"type":"provision","registrationEndpoint":"http://testbench:3011/${CTX:apiPortalRunnerPartition}/register","auth":{"method":"basic","username":"it-provisioner","password":"${CTX:provisioningPassword}"}} |
    When a unique API Portal resource is created at "/oauth2-keys" as "developer" with values and stored as "keyId":
      | keyManagerId | ${CTX:kmId}                                              |
      | properties   | {"client_name":"Imported client","consumerKey":"test-client"} |
    Then the response status code should be 201
    And the JSON response field "consumerKey" should be "test-client"
    And the JSON response field "keyManagerId" should be "${CTX:kmId}"
    And the JSON response field "name" should be "Imported client"
    And the response body should not contain "test-secret"
    When I send an authenticated API Portal "GET" request to "/oauth2-keys/${CTX:keyId}" as "developer"
    Then the response status code should be 200
    And the JSON response field "consumerKey" should be "test-client"
    And the response body should not contain "test-secret"
    When I send an authenticated API Portal "GET" request to "/oauth2-keys" as "developer"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "keyId" equal to "${CTX:keyId}"
    When I send an authenticated API Portal "POST" request to "/oauth2-keys/${CTX:keyId}/generate-token" as "developer" with JSON values:
      | consumerSecret | test-secret |
    Then the response status code should be 200
    And the response body should contain "mock-token"
    And the response body should not contain "test-secret"

    # Another user cannot read, use or delete the key.
    When I send an authenticated API Portal "GET" request to "/oauth2-keys/${CTX:keyId}" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "POST" request to "/oauth2-keys/${CTX:keyId}/generate-token" as "admin" with JSON values:
      | consumerSecret | test-secret |
    Then the response status code should be 404
    When I send an authenticated API Portal "DELETE" request to "/oauth2-keys/${CTX:keyId}" as "admin"
    Then the response status code should be 404

    When I send an authenticated API Portal "DELETE" request to "/oauth2-keys/${CTX:keyId}" as "developer"
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/oauth2-keys/${CTX:keyId}" as "developer"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/oauth2-keys" as "developer"
    Then the JSON response array "list" should not contain an item with "keyId" equal to "${CTX:keyId}"

    # An OAuth2 key is linked to its owner's application and unlinked, and cannot be linked to another user's
    Given I generate a unique resource name from "portal-link-km" and store it as "kmId"
    And I generate a unique value from "portal-km-password" and store it as "provisioningPassword"
    And I generate a unique resource name from "portal-link-app" and store it as "appId"
    And I generate a unique resource name from "portal-link-admin-app" and store it as "adminAppId"
    And a unique API Portal resource is created at "/key-managers" as "admin" with values and stored as "kmId":
      | id            | ${CTX:kmId}                                                          |
      | displayName   | Link Key Manager                                                     |
      | tokenEndpoint | http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token |
      | provisioning  | {"type":"provision","registrationEndpoint":"http://testbench:3011/${CTX:apiPortalRunnerPartition}/register","auth":{"method":"basic","username":"it-provisioner","password":"${CTX:provisioningPassword}"}} |
    And a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}       |
      | displayName | Linked Application |
      | description | Holds an OAuth2 key |
    And a unique API Portal resource is created at "/applications" as "admin" with values and stored as "adminAppId":
      | id          | ${CTX:adminAppId}      |
      | displayName | Administrator App      |
      | description | Owned by someone else  |
    And a unique API Portal resource is created at "/oauth2-keys" as "developer" with values and stored as "keyId":
      | keyManagerId | ${CTX:kmId}                                            |
      | properties   | {"client_name":"Linked client","consumerKey":"test-client"} |

    When I send an authenticated API Portal "POST" request to "/oauth2-keys/${CTX:keyId}/associate" as "developer" with JSON values:
      | applicationId | ${CTX:adminAppId} |
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/applications/${CTX:adminAppId}/oauth2-keys" as "admin"
    Then the response status code should be 200
    And the JSON response array "list" should not contain an item with "keyId" equal to "${CTX:keyId}"

    When I send an authenticated API Portal "POST" request to "/oauth2-keys/${CTX:keyId}/associate" as "developer" with JSON values:
      | applicationId | ${CTX:appId} |
    Then the response status code should be 200
    And the JSON response field "application.id" should be "${CTX:appId}"
    And the JSON response field "application.displayName" should be "Linked Application"
    When I send an authenticated API Portal "GET" request to "/applications/${CTX:appId}/oauth2-keys" as "developer"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "keyId" equal to "${CTX:keyId}"

    When I send an authenticated API Portal "POST" request to "/oauth2-keys/${CTX:keyId}/dissociate" as "developer"
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/applications/${CTX:appId}/oauth2-keys" as "developer"
    Then the response status code should be 200
    And the JSON response array "list" should not contain an item with "keyId" equal to "${CTX:keyId}"
    When I send an authenticated API Portal "GET" request to "/oauth2-keys/${CTX:keyId}" as "developer"
    Then the response status code should be 200
