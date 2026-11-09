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


@APIPORTAL @apiportal-key-managers
Feature: Administrators register the key managers developers get OAuth2 tokens from
  As an API Portal administrator
  I want to register, validate, disable and remove key managers safely
  So that developers get tokens only from identity providers I have enabled, and the portal never reaches an address it must not

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml), IDs APIP-CORE-170..177, at the API level.
  # Workbook row APIP-015 steps 1, 2, 3, 5 and 7; steps 1 and 6 are also covered by APIP-CORE-029
  # and APIP-CORE-031. Key managers point at this scenario's partition of the testbench OAuth2 mock,
  # whose fixed client is "test-client" / "test-secret" — a mock credential, never a real one. A
  # provisioning credential is generated per run and must never come back in a response.

  @APIP-CORE-170 @type:security
  Scenario: A key manager registered without an id gets one, and its provisioning credential is never returned
    Given I generate a unique value from "portal-km-password" and store it as "provisioningPassword"
    When a unique API Portal resource is created at "/key-managers" as "admin" with values and stored as "kmId":
      | displayName   | Provisioning Key Manager                                             |
      | tokenEndpoint | http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token |
      | provisioning  | {"type":"provision","registrationEndpoint":"http://testbench:3011/${CTX:apiPortalRunnerPartition}/register","auth":{"method":"basic","username":"it-provisioner","password":"${CTX:provisioningPassword}"}} |
    Then the response status code should be 201
    And the JSON response field "id" should match pattern "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"
    And the response body should not contain "${CTX:provisioningPassword}"
    When I send an authenticated API Portal "GET" request to "/key-managers/${CTX:kmId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Provisioning Key Manager"
    And the response body should not contain "${CTX:provisioningPassword}"
    When I send an authenticated API Portal "GET" request to "/key-managers" as "admin"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:kmId}"
    And the response body should not contain "${CTX:provisioningPassword}"
    When I send an authenticated API Portal "GET" request to "/key-managers/metadata" as "developer"
    Then the response status code should be 200
    And the response body should not contain "${CTX:provisioningPassword}"

  @APIP-CORE-171 @type:validation
  Scenario: A key manager with a taken id, no token endpoint or an invalid id is refused, and an unknown one cannot be updated
    Given I generate a unique resource name from "portal-km-taken" and store it as "kmId"
    And I generate a unique resource name from "portal-km-no-endpoint" and store it as "noEndpointKmId"
    And I generate a unique resource name from "portal-km-unknown" and store it as "unknownKmId"
    And a unique API Portal resource is created at "/key-managers" as "admin" with values and stored as "kmId":
      | id            | ${CTX:kmId}                                                          |
      | displayName   | Orders Key Manager                                                   |
      | tokenEndpoint | http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token |
    When I send an authenticated API Portal "POST" request to "/key-managers" as "admin" with JSON values:
      | id            | ${CTX:kmId}                                                          |
      | displayName   | Second Key Manager                                                   |
      | tokenEndpoint | http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token |
    Then the response status code should be 409
    And the response body should not contain "stack"
    When I send an authenticated API Portal "POST" request to "/key-managers" as "admin" with JSON values:
      | id          | ${CTX:noEndpointKmId} |
      | displayName | No Endpoint           |
    Then the response status code should be 400
    When I send an authenticated API Portal "GET" request to "/key-managers/${CTX:noEndpointKmId}" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "POST" request to "/key-managers" as "admin" with JSON values:
      | id            | Invalid id!                                                          |
      | displayName   | Invalid Id                                                           |
      | tokenEndpoint | http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token |
    Then the response status code should be 400
    When I send an authenticated API Portal "PUT" request to "/key-managers/${CTX:unknownKmId}" as "admin" with JSON values:
      | displayName | Never Created |
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/key-managers/${CTX:kmId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Orders Key Manager"

  @APIP-CORE-172
  Scenario: Discovery reads nothing from a URL without a discovery document, and metadata describes each enabled key manager
    Given I generate a unique resource name from "portal-km-metadata" and store it as "kmId"
    And I generate a unique value from "portal-km-password" and store it as "provisioningPassword"
    And a unique API Portal resource is created at "/key-managers" as "admin" with values and stored as "kmId":
      | id            | ${CTX:kmId}                                                          |
      | displayName   | Import Key Manager                                                   |
      | tokenEndpoint | http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token |
      | provisioning  | {"type":"provision","registrationEndpoint":"http://testbench:3011/${CTX:apiPortalRunnerPartition}/register","auth":{"method":"basic","username":"it-provisioner","password":"${CTX:provisioningPassword}"}} |
    When I send an authenticated API Portal "POST" request to "/key-managers/discovery" as "admin" with JSON values:
      | url | http://testbench:3011/${CTX:apiPortalRunnerPartition}/.well-known/openid-configuration |
    Then the response status code should be 400
    And the response body should not contain "stack"
    And the response body should not contain "testbench"
    When I send an authenticated API Portal "POST" request to "/key-managers/discovery" as "developer" with JSON values:
      | url | http://testbench:3011/${CTX:apiPortalRunnerPartition}/.well-known/openid-configuration |
    Then the response status code should be 403
    When I send an authenticated API Portal "GET" request to "/key-managers/metadata" as "developer"
    Then the response status code should be 200
    And the JSON response array "list" item with "id" equal to "${CTX:kmId}" should have "type" equal to "provision"
    And the JSON response array "list" item with "id" equal to "${CTX:kmId}" should have "keyCreation" equal to "provide"
    And the JSON response array "list" item with "id" equal to "${CTX:kmId}" should have "displayName" equal to "Import Key Manager"

  @APIP-CORE-173
  Scenario: A disabled key manager is hidden and issues no token through its keys, until it is enabled again
    Given I generate a unique resource name from "portal-km-disabled" and store it as "kmId"
    And I generate a unique value from "portal-km-password" and store it as "provisioningPassword"
    And a unique API Portal resource is created at "/key-managers" as "admin" with values and stored as "kmId":
      | id            | ${CTX:kmId}                                                          |
      | displayName   | Switchable Key Manager                                               |
      | tokenEndpoint | http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token |
      | provisioning  | {"type":"provision","registrationEndpoint":"http://testbench:3011/${CTX:apiPortalRunnerPartition}/register","auth":{"method":"basic","username":"it-provisioner","password":"${CTX:provisioningPassword}"}} |
    And a unique API Portal resource is created at "/oauth2-keys" as "developer" with values and stored as "keyId":
      | keyManagerId | ${CTX:kmId}                                               |
      | properties   | {"client_name":"Switchable client","consumerKey":"test-client"} |
    When I send an authenticated API Portal "POST" request to "/oauth2-keys/${CTX:keyId}/generate-token" as "developer" with JSON values:
      | consumerSecret | test-secret |
    Then the response status code should be 200
    And the response body should contain "mock-token"

    When I send an authenticated API Portal "PUT" request to "/key-managers/${CTX:kmId}" as "admin" with JSON body:
      """
      {"enabled": false}
      """
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/key-managers" as "developer"
    Then the response status code should be 200
    And the JSON response array "list" should not contain an item with "id" equal to "${CTX:kmId}"
    When I send an authenticated API Portal "GET" request to "/key-managers/metadata" as "developer"
    Then the JSON response array "list" should not contain an item with "id" equal to "${CTX:kmId}"
    When I send an authenticated API Portal "POST" request to "/oauth2-keys/${CTX:keyId}/generate-token" as "developer" with JSON values:
      | consumerSecret | test-secret |
    Then the response status code should be 409
    And the response body should not contain "mock-token"
    And the response body should not contain "test-secret"

    When I send an authenticated API Portal "PUT" request to "/key-managers/${CTX:kmId}" as "admin" with JSON body:
      """
      {"enabled": true}
      """
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/key-managers" as "developer"
    Then the JSON response array "list" should contain an item with "id" equal to "${CTX:kmId}"
    When I send an authenticated API Portal "POST" request to "/oauth2-keys/${CTX:keyId}/generate-token" as "developer" with JSON values:
      | consumerSecret | test-secret |
    Then the response status code should be 200
    And the response body should contain "mock-token"

  @APIP-CORE-174 @known-issue
  Scenario: A disabled key manager issues no token through an application's client mapping
    # Known issue: POST /applications/{id}/oauth-keys/{mappingId}/generate-token never checks whether the key manager is enabled.
    Given I generate a unique resource name from "portal-km-disabled-mapping" and store it as "kmId"
    And I generate a unique resource name from "portal-km-disabled-app" and store it as "appId"
    And a unique API Portal resource is created at "/key-managers" as "admin" with values and stored as "kmId":
      | id            | ${CTX:kmId}                                                          |
      | displayName   | Disabled Mapping Key Manager                                         |
      | tokenEndpoint | http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token |
    And a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}         |
      | displayName | Mapping Application  |
      | description | Uses a disabled KM   |
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
    When I send an authenticated API Portal "PUT" request to "/key-managers/${CTX:kmId}" as "admin" with JSON body:
      """
      {"enabled": false}
      """
    Then the response status code should be 200
    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/oauth-keys/${CTX:keyMappingId}/generate-token" as "developer" with JSON values:
      | consumerSecret | test-secret |
    Then the response should be a client error
    And the response body should not contain "mock-token"

  @APIP-CORE-175 @known-issue
  Scenario: A deleted key manager issues no more tokens to the applications mapped to it
    # Known issue: deleting a key manager that an application's client mapping still references answers 500 and keeps it.
    Given I generate a unique resource name from "portal-km-deleted" and store it as "kmId"
    And I generate a unique resource name from "portal-km-deleted-app" and store it as "appId"
    And a unique API Portal resource is created at "/key-managers" as "admin" with values and stored as "kmId":
      | id            | ${CTX:kmId}                                                          |
      | displayName   | Deleted Key Manager                                                  |
      | tokenEndpoint | http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token |
    And a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}          |
      | displayName | Orphaned Application  |
      | description | Loses its key manager |
    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/generate-keys" as "developer" with JSON values:
      | keyManager  | ${CTX:kmId} |
      | type        | PRODUCTION  |
      | consumerKey | test-client |
    Then the response status code should be 200
    And I store the JSON response field "keyMappingId" as "keyMappingId"
    And I register API Portal application key mapping "${CTX:keyMappingId}" for application "${CTX:appId}" for cleanup
    When I send an authenticated API Portal "DELETE" request to "/key-managers/${CTX:kmId}" as "admin"
    Then the response status code should be 204
    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/oauth-keys/${CTX:keyMappingId}/generate-token" as "developer" with JSON values:
      | consumerSecret | test-secret |
    Then the response should be a client error
    And the response body should not contain "mock-token"

  @APIP-CORE-176 @type:security
  Scenario Outline: A key manager or discovery URL at <target> is refused and nothing is stored
    Given I generate a unique resource name from "portal-km-ssrf" and store it as "deniedKmId"
    And I generate a unique resource name from "portal-km-allowed" and store it as "kmId"
    When I send an authenticated API Portal "POST" request to "/key-managers" as "admin" with JSON values:
      | id            | ${CTX:deniedKmId} |
      | displayName   | Denied Endpoint   |
      | tokenEndpoint | <url>             |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/key-managers/${CTX:deniedKmId}" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "POST" request to "/key-managers/discovery" as "admin" with JSON values:
      | url | <url> |
    Then the response status code should be 400
    And the response body should not contain "stack"
    And a unique API Portal resource is created at "/key-managers" as "admin" with values and stored as "kmId":
      | id            | ${CTX:kmId}                                                          |
      | displayName   | Allowed Endpoint                                                     |
      | tokenEndpoint | http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token |

    Examples:
      | target                    | url                                         |
      | the cloud metadata address | http://169.254.169.254/latest/meta-data/    |
      | a local file              | file:///etc/passwd                          |
