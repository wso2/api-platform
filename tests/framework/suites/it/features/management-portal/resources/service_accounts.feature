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
@platform-api-service-accounts
Feature: Service accounts
  As an organization administrator
  I want a workload to sign in with a client ID and secret
  So that it can call the Platform API without borrowing a person's credentials

  # Every runner in the block shares one organization, so accounts are named from generated
  # values and found only by those names. A token is used through the control-plane request
  # step with its Authorization header overridden.

  @cap:service-accounts @feat:account-lifecycle @type:smoke
  Scenario: An account is created, read, changed, listed and deleted
    Given I generate a unique resource name from "sa-life" and store it as "saName"
    When I create a service account via the control plane from "resources/templates/service-account.yaml" with values:
      | displayName | ${CTX:saName}          |
      | roles       | ["ap_service_account"] |
    Then the response status code should be 201
    And I store the JSON response field "serviceAccount.id" as "saId"
    And I store the JSON response field "clientId" as "clientId"
    And I store the JSON response field "clientSecret" as "clientSecret"
    And the response header "Location" should be "/api/v0.9/service-accounts/${CTX:saId}"
    And the JSON response field "serviceAccount.clientId" should be "${CTX:clientId}"
    And the response body should match pattern "clientId.{3}sa_[\w-]+_[0-9a-f]{6}\W"
    And the JSON response field "clientId" should contain "_${CTX:saId}_"
    And the response body should match pattern "clientSecret.{3}apsa_[0-9a-f]{64}[^0-9a-f]"
    And the JSON response field "serviceAccount.clientSecret" should not exist
    And the response body should match pattern "maskedSecret.{3}\*\*\*[0-9a-f]{5}\W"
    And the JSON response field "serviceAccount.displayName" should be "${CTX:saName}"
    And the JSON response field "serviceAccount.description" should be ""
    And the JSON response array field "serviceAccount.roles" should have 1 item
    And the JSON response field "serviceAccount.roles[0]" should be "ap_service_account"
    And the JSON response field "serviceAccount.status" should be "active"
    And the JSON response field "serviceAccount.createdBy" should be "admin"
    When I send a "GET" request to the control plane at "/service-accounts/${CTX:saId}"
    Then the response status code should be 200
    And the JSON response field "id" should be "${CTX:saId}"
    And the JSON response field "clientId" should be "${CTX:clientId}"
    And the response body should match pattern "maskedSecret.{3}\*\*\*[0-9a-f]{5}\W"
    And the JSON response field "clientSecret" should not exist
    And the JSON response field "lastUsedAt" should not exist
    And the response body should not contain "${CTX:clientSecret}"
    When I send a "GET" request to the control plane at "/service-accounts?query=${CTX:saName}"
    Then the response status code should be 200
    And the JSON response field "count" should be 1
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:saId}"
    And the response body should not contain "${CTX:clientSecret}"
    When I send a "GET" request to the control plane at "/roles"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "name" equal to "ap_service_account"
    And the JSON response array "list" should contain an item with "name" equal to "ap_admin"
    When I update the service account "${CTX:saId}" via the control plane from "resources/templates/service-account.yaml" with values:
      | displayName | Renamed ${CTX:saName}                    |
      | description | Reads REST APIs for the release pipeline |
      | roles       | ["ap_service_account", "ap_viewer"]      |
      | status      | active                                   |
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/service-accounts/${CTX:saId}"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Renamed ${CTX:saName}"
    And the JSON response field "description" should be "Reads REST APIs for the release pipeline"
    And the JSON response array field "roles" should have 2 items
    And the JSON response field "roles[0]" should be "ap_service_account"
    And the JSON response field "roles[1]" should be "ap_viewer"
    And the JSON response field "status" should be "active"
    And the JSON response field "updatedBy" should be "admin"
    And the JSON response field "clientId" should be "${CTX:clientId}"
    When I send a "DELETE" request to the control plane at "/service-accounts/${CTX:saId}"
    Then the response status code should be 204
    When I send a "GET" request to the control plane at "/service-accounts/${CTX:saId}"
    Then the response status code should be 404
    And the JSON response field "code" should be "SERVICE_ACCOUNT_NOT_FOUND"
    When I send a "DELETE" request to the control plane at "/service-accounts/${CTX:saId}"
    Then the response status code should be 404
    And the JSON response field "code" should be "SERVICE_ACCOUNT_NOT_FOUND"

  @cap:service-accounts @feat:token-exchange @type:smoke
  Scenario: Credentials are exchanged for a token in the request body and with HTTP Basic
    Given I generate a unique resource name from "sa-exchange" and store it as "saName"
    And I create a service account via the control plane from "resources/templates/service-account.yaml" with values:
      | displayName | ${CTX:saName}          |
      | roles       | ["ap_service_account"] |
    And the response status code should be 201
    And I store the JSON response field "serviceAccount.id" as "saId"
    And I store the JSON response field "clientId" as "clientId"
    And I store the JSON response field "clientSecret" as "clientSecret"
    When I exchange service account credentials with values:
      | client_id     | ${CTX:clientId}                  |
      | client_secret | ${CTX:clientSecret}              |
      | scope         | ap:rest_api:read ap:gateway:read |
    Then the response status code should be 200
    And the response header "Cache-Control" should be "no-store"
    And the JSON response field "token_type" should be "Bearer"
    And the JSON response field "expires_in" should be 900
    And the JSON response field "scope" should be "ap:rest_api:read ap:gateway:read"
    And the JWT in the JSON response field "access_token" should have header "alg" equal to "RS256"
    And the JWT in the JSON response field "access_token" should have header "kid" matching "^[A-Za-z0-9_-]{43}$"
    And the JWT in the JSON response field "access_token" should have claim "iss" equal to "platform-api"
    And the JWT in the JSON response field "access_token" should have claim "aud" equal to "platform-api"
    And the JWT in the JSON response field "access_token" should have claim "azp" equal to "${CTX:clientId}"
    And the JWT in the JSON response field "access_token" should have claim "sub" matching "^sa:[^:]+:${CTX:saId}:[0-9a-f-]{36}$"
    And the JWT in the JSON response field "access_token" should have claim "scope" equal to "ap:rest_api:read ap:gateway:read"
    When I exchange service account credentials using HTTP Basic as "${CTX:clientId}" with secret "${CTX:clientSecret}" with values:
      | scope | ap:rest_api:read ap:gateway:read |
    Then the response status code should be 200
    And the JSON response field "token_type" should be "Bearer"
    And the JSON response field "expires_in" should be 900
    And the JSON response field "scope" should be "ap:rest_api:read ap:gateway:read"
    And the JWT in the JSON response field "access_token" should have claim "azp" equal to "${CTX:clientId}"
    When I send a "GET" request to the control plane at "/service-accounts/${CTX:saId}"
    Then the response status code should be 200
    And the JSON response should have field "lastUsedAt"
    And the JSON response should have field "lastUsedIp"

  @cap:service-accounts @feat:token-use
  Scenario: A token works inside its requested scope and nowhere else
    Given I generate a unique resource name from "sa-use" and store it as "saName"
    And I generate a unique resource name from "sa-use-project" and store it as "project"
    And I create a project "${CTX:project}" on the control plane
    And I create a service account via the control plane from "resources/templates/service-account.yaml" with values:
      | displayName | ${CTX:saName}          |
      | roles       | ["ap_service_account"] |
    And the response status code should be 201
    And I store the JSON response field "clientId" as "clientId"
    And I store the JSON response field "clientSecret" as "clientSecret"
    And I exchange service account credentials with values:
      | client_id     | ${CTX:clientId}     |
      | client_secret | ${CTX:clientSecret} |
      | scope         | ap:rest_api:read    |
    And the response status code should be 200
    And I store the JSON response field "access_token" as "token"
    When I send a "GET" request to the control plane at "/rest-apis?projectId=${CTX:project}" with header "Authorization" set to "Bearer ${CTX:token}"
    Then the response status code should be 200
    And the JSON response field "count" should be 0
    # The account's role grants these two reads; the token was narrowed to one scope.
    When I send a "GET" request to the control plane at "/gateways" with header "Authorization" set to "Bearer ${CTX:token}"
    Then the response status code should be 403
    And the JSON response field "code" should be "FORBIDDEN"
    When I send a "GET" request to the control plane at "/projects" with header "Authorization" set to "Bearer ${CTX:token}"
    Then the response status code should be 403
    And the JSON response field "code" should be "FORBIDDEN"
    # The role grants no write; the scope check answers before the body is read.
    When I send a "POST" request to the control plane at "/projects" with header "Authorization" set to "Bearer ${CTX:token}"
    Then the response status code should be 403
    And the JSON response field "code" should be "FORBIDDEN"
    When I send a "GET" request to the control plane at "/rest-apis?projectId=${CTX:project}" with header "Authorization" set to "Bearer garbage"
    Then the response status code should be 401
    And the JSON response field "code" should be "UNAUTHORIZED"
    When I send a "GET" request to the control plane at "/rest-apis?projectId=${CTX:project}" with header "Authorization" set to "Bearer aaa.bbb.ccc"
    Then the response status code should be 401
    And the JSON response field "code" should be "UNAUTHORIZED"

  @cap:service-accounts @feat:token-exchange @type:negative
  Scenario Outline: The exchange refuses <case>
    Given I generate a unique resource name from "sa-refuse" and store it as "saName"
    And I create a service account via the control plane from "resources/templates/service-account.yaml" with values:
      | displayName | ${CTX:saName}          |
      | roles       | ["ap_service_account"] |
    And the response status code should be 201
    And I store the JSON response field "clientId" as "clientId"
    And I store the JSON response field "clientSecret" as "clientSecret"
    When I exchange service account credentials with values:
      | grant_type    | <grant_type>    |
      | client_id     | <client_id>     |
      | client_secret | <client_secret> |
      | scope         | <scope>         |
    Then the response status code should be <status>
    And the JSON response field "code" should be "<code>"
    And the JSON response field "message" should be "<message>"
    And the JSON response field "access_token" should not exist

    Examples:
      | case                           | grant_type         | client_id               | client_secret       | scope                     | status | code                          | message                                                                    |
      | a wrong secret                 | client_credentials | ${CTX:clientId}         | apsa_wrong          | ap:project:read           | 401    | UNAUTHORIZED                  | Invalid or expired credentials.                                            |
      | an unknown client              | client_credentials | sa_nobody_${CTX:saName} | ${CTX:clientSecret} | ap:project:read           | 401    | UNAUTHORIZED                  | Invalid or expired credentials.                                            |
      | a request with no scope        | client_credentials | ${CTX:clientId}         | ${CTX:clientSecret} |                           | 400    | SERVICE_ACCOUNT_INVALID_SCOPE | The requested scope is missing, or is not granted to this service account. |
      | a scope the roles do not grant | client_credentials | ${CTX:clientId}         | ${CTX:clientSecret} | ap:service_account:manage | 400    | SERVICE_ACCOUNT_INVALID_SCOPE | The requested scope is missing, or is not granted to this service account. |
      | another grant type             | password           | ${CTX:clientId}         | ${CTX:clientSecret} | ap:project:read           | 400    | VALIDATION_FAILED             | grant_type must be client_credentials                                      |

  @cap:service-accounts @feat:token-exchange @type:negative
  Scenario: The exchange refuses a client that authenticates twice, and a wrong HTTP Basic secret
    Given I generate a unique resource name from "sa-twice" and store it as "saName"
    And I create a service account via the control plane from "resources/templates/service-account.yaml" with values:
      | displayName | ${CTX:saName}          |
      | roles       | ["ap_service_account"] |
    And the response status code should be 201
    And I store the JSON response field "clientId" as "clientId"
    And I store the JSON response field "clientSecret" as "clientSecret"
    When I exchange service account credentials using HTTP Basic as "${CTX:clientId}" with secret "${CTX:clientSecret}" with values:
      | client_id | ${CTX:clientId} |
      | scope     | ap:project:read |
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"
    And the JSON response field "message" should be "use one client authentication method: HTTP Basic or form fields, not both"
    When I exchange service account credentials using HTTP Basic as "${CTX:clientId}" with secret "apsa_wrong" with values:
      | scope | ap:project:read |
    Then the response status code should be 401
    And the JSON response field "code" should be "UNAUTHORIZED"
    And the JSON response field "message" should be "Invalid or expired credentials."

  @cap:service-accounts @feat:token-exchange @type:negative
  Scenario: The exchange refuses a request that leaves out the scope
    Given I generate a unique resource name from "sa-noscope" and store it as "saName"
    And I create a service account via the control plane from "resources/templates/service-account.yaml" with values:
      | displayName | ${CTX:saName}          |
      | roles       | ["ap_service_account"] |
    And the response status code should be 201
    And I store the JSON response field "clientId" as "clientId"
    And I store the JSON response field "clientSecret" as "clientSecret"
    When I exchange service account credentials with values:
      | client_id     | ${CTX:clientId}     |
      | client_secret | ${CTX:clientSecret} |
    Then the response status code should be 400
    And the JSON response field "code" should be "SERVICE_ACCOUNT_INVALID_SCOPE"
    And the JSON response field "access_token" should not exist

  @cap:service-accounts @feat:account-lifecycle @type:negative
  Scenario: A service account cannot manage service accounts, even with the administrator role
    Given I generate a unique resource name from "sa-selfmanage" and store it as "saName"
    And I create a service account via the control plane from "resources/templates/service-account.yaml" with values:
      | displayName | ${CTX:saName} |
      | roles       | ["ap_admin"]  |
    And the response status code should be 201
    And I store the JSON response field "serviceAccount.id" as "saId"
    And I store the JSON response field "clientId" as "clientId"
    And I store the JSON response field "clientSecret" as "clientSecret"
    When I exchange service account credentials with values:
      | client_id     | ${CTX:clientId}           |
      | client_secret | ${CTX:clientSecret}       |
      | scope         | ap:service_account:manage |
    Then the response status code should be 200
    And the JSON response field "scope" should be "ap:service_account:manage"
    And I store the JSON response field "access_token" as "token"
    When I send a "GET" request to the control plane at "/service-accounts" with header "Authorization" set to "Bearer ${CTX:token}"
    Then the response status code should be 403
    And the JSON response field "code" should be "FORBIDDEN"
    When I send a "POST" request to the control plane at "/service-accounts/${CTX:saId}/regenerate-secret" with header "Authorization" set to "Bearer ${CTX:token}"
    Then the response status code should be 403
    And the JSON response field "code" should be "FORBIDDEN"
    When I exchange service account credentials with values:
      | client_id     | ${CTX:clientId}     |
      | client_secret | ${CTX:clientSecret} |
      | scope         | ap:project:manage   |
    Then the response status code should be 200

  @cap:service-accounts @feat:account-lifecycle @type:negative
  Scenario Outline: Nobody manages service accounts without signing in: <method> <path>
    When I send a "<method>" request to the control plane at "<path>" without signing in
    Then the response status code should be 401
    And the JSON response field "code" should be "UNAUTHORIZED"

    Examples:
      | method | path                                       |
      | GET    | /service-accounts                          |
      | POST   | /service-accounts                          |
      | GET    | /service-accounts/nobody                   |
      | DELETE | /service-accounts/nobody                   |
      | POST   | /service-accounts/nobody/regenerate-secret |
      | GET    | /roles                                     |

  @cap:service-accounts @feat:account-lifecycle @type:negative
  Scenario Outline: Only an administrator manages service accounts: <user> is refused <method> <path>
    When I send a "<method>" request to the control plane at "<path>" as "<user>"
    Then the response status code should be 403
    And the JSON response field "code" should be "FORBIDDEN"

    Examples:
      | user      | method | path              |
      | publisher | POST   | /service-accounts |
      | publisher | GET    | /service-accounts |
      | publisher | GET    | /roles            |
      | developer | POST   | /service-accounts |
      | developer | GET    | /service-accounts |
      | developer | GET    | /roles            |

  @cap:service-accounts @feat:revocation
  Scenario: Disabling an account stops its tokens, and enabling it again does not bring them back
    Given I generate a unique resource name from "sa-disable" and store it as "saName"
    And I create a service account via the control plane from "resources/templates/service-account.yaml" with values:
      | displayName | ${CTX:saName}          |
      | roles       | ["ap_service_account"] |
    And the response status code should be 201
    And I store the JSON response field "serviceAccount.id" as "saId"
    And I store the JSON response field "clientId" as "clientId"
    And I store the JSON response field "clientSecret" as "clientSecret"
    When I exchange service account credentials with values:
      | client_id     | ${CTX:clientId}     |
      | client_secret | ${CTX:clientSecret} |
      | scope         | ap:project:read     |
    Then the response status code should be 200
    And I store the JSON response field "access_token" as "tokenA"
    When I send a "GET" request to the control plane at "/projects?limit=1" with header "Authorization" set to "Bearer ${CTX:tokenA}"
    Then the response status code should be 200
    When I update the service account "${CTX:saId}" via the control plane from "resources/templates/service-account.yaml" with values:
      | displayName | ${CTX:saName}          |
      | roles       | ["ap_service_account"] |
      | status      | disabled               |
    Then the response status code should be 200
    And the JSON response field "status" should be "disabled"
    When I send a "GET" request to the control plane at "/projects?limit=1" with header "Authorization" set to "Bearer ${CTX:tokenA}"
    Then the response status code should be 401
    And the JSON response field "code" should be "UNAUTHORIZED"
    When I exchange service account credentials with values:
      | client_id     | ${CTX:clientId}     |
      | client_secret | ${CTX:clientSecret} |
      | scope         | ap:project:read     |
    Then the response status code should be 401
    And the JSON response field "code" should be "UNAUTHORIZED"
    When I update the service account "${CTX:saId}" via the control plane from "resources/templates/service-account.yaml" with values:
      | displayName | ${CTX:saName}          |
      | roles       | ["ap_service_account"] |
      | status      | active                 |
    Then the response status code should be 200
    And the JSON response field "status" should be "active"
    When I send a "GET" request to the control plane at "/projects?limit=1" with header "Authorization" set to "Bearer ${CTX:tokenA}"
    Then the response status code should be 401
    And the JSON response field "code" should be "UNAUTHORIZED"
    When I exchange service account credentials with values:
      | client_id     | ${CTX:clientId}     |
      | client_secret | ${CTX:clientSecret} |
      | scope         | ap:project:read     |
    Then the response status code should be 200
    And I store the JSON response field "access_token" as "tokenB"
    When I send a "GET" request to the control plane at "/projects?limit=1" with header "Authorization" set to "Bearer ${CTX:tokenB}"
    Then the response status code should be 200

  @cap:service-accounts @feat:revocation @type:regression
  Scenario: Regenerating the secret replaces it at once, and the new secret's first token works
    Given I generate a unique resource name from "sa-regen" and store it as "saName"
    And I create a service account via the control plane from "resources/templates/service-account.yaml" with values:
      | displayName | ${CTX:saName}          |
      | roles       | ["ap_service_account"] |
    And the response status code should be 201
    And I store the JSON response field "serviceAccount.id" as "saId"
    And I store the JSON response field "clientId" as "clientId"
    And I store the JSON response field "clientSecret" as "clientSecret"
    And I store the JSON response field "serviceAccount.maskedSecret" as "maskedBefore"
    When I exchange service account credentials with values:
      | client_id     | ${CTX:clientId}     |
      | client_secret | ${CTX:clientSecret} |
      | scope         | ap:project:read     |
    Then the response status code should be 200
    And I store the JSON response field "access_token" as "tokenA"
    When I send a "GET" request to the control plane at "/projects?limit=1" with header "Authorization" set to "Bearer ${CTX:tokenA}"
    Then the response status code should be 200
    When I send a "POST" request to the control plane at "/service-accounts/${CTX:saId}/regenerate-secret"
    Then the response status code should be 200
    And the JSON response field "clientId" should be "${CTX:clientId}"
    And the JSON response field "serviceAccount.maskedSecret" should not equal "${CTX:maskedBefore}"
    And the JSON response field "clientSecret" should not equal "${CTX:clientSecret}"
    And the response body should match pattern "clientSecret.{3}apsa_[0-9a-f]{64}[^0-9a-f]"
    And I store the JSON response field "clientSecret" as "newSecret"
    When I exchange service account credentials with values:
      | client_id     | ${CTX:clientId}     |
      | client_secret | ${CTX:clientSecret} |
      | scope         | ap:project:read     |
    Then the response status code should be 401
    And the JSON response field "code" should be "UNAUTHORIZED"
    When I send a "GET" request to the control plane at "/projects?limit=1" with header "Authorization" set to "Bearer ${CTX:tokenA}"
    Then the response status code should be 401
    And the JSON response field "code" should be "UNAUTHORIZED"
    When I exchange service account credentials with values:
      | client_id     | ${CTX:clientId}  |
      | client_secret | ${CTX:newSecret} |
      | scope         | ap:project:read  |
    Then the response status code should be 200
    And I store the JSON response field "access_token" as "tokenB"
    When I send a "GET" request to the control plane at "/projects?limit=1" with header "Authorization" set to "Bearer ${CTX:tokenB}"
    Then the response status code should be 200

  @cap:service-accounts @feat:revocation
  Scenario: Removing a role stops issued tokens, adding one does not, and deleting the account stops them all
    Given I generate a unique resource name from "sa-roles" and store it as "saName"
    And I create a service account via the control plane from "resources/templates/service-account.yaml" with values:
      | displayName | ${CTX:saName}                       |
      | roles       | ["ap_service_account", "ap_viewer"] |
    And the response status code should be 201
    And I store the JSON response field "serviceAccount.id" as "saId"
    And I store the JSON response field "clientId" as "clientId"
    And I store the JSON response field "clientSecret" as "clientSecret"
    When I exchange service account credentials with values:
      | client_id     | ${CTX:clientId}     |
      | client_secret | ${CTX:clientSecret} |
      | scope         | ap:project:read     |
    Then the response status code should be 200
    And I store the JSON response field "access_token" as "tokenA"
    When I send a "GET" request to the control plane at "/projects?limit=1" with header "Authorization" set to "Bearer ${CTX:tokenA}"
    Then the response status code should be 200
    When I update the service account "${CTX:saId}" via the control plane from "resources/templates/service-account.yaml" with values:
      | displayName | ${CTX:saName}                                       |
      | roles       | ["ap_service_account", "ap_viewer", "ap_publisher"] |
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/projects?limit=1" with header "Authorization" set to "Bearer ${CTX:tokenA}"
    Then the response status code should be 200
    When I update the service account "${CTX:saId}" via the control plane from "resources/templates/service-account.yaml" with values:
      | displayName | ${CTX:saName}          |
      | roles       | ["ap_service_account"] |
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/projects?limit=1" with header "Authorization" set to "Bearer ${CTX:tokenA}"
    Then the response status code should be 401
    And the JSON response field "code" should be "UNAUTHORIZED"
    When I exchange service account credentials with values:
      | client_id     | ${CTX:clientId}     |
      | client_secret | ${CTX:clientSecret} |
      | scope         | ap:project:read     |
    Then the response status code should be 200
    And I store the JSON response field "access_token" as "tokenB"
    When I send a "GET" request to the control plane at "/projects?limit=1" with header "Authorization" set to "Bearer ${CTX:tokenB}"
    Then the response status code should be 200
    When I send a "DELETE" request to the control plane at "/service-accounts/${CTX:saId}"
    Then the response status code should be 204
    When I send a "GET" request to the control plane at "/projects?limit=1" with header "Authorization" set to "Bearer ${CTX:tokenB}"
    Then the response status code should be 401
    And the JSON response field "code" should be "UNAUTHORIZED"
    When I exchange service account credentials with values:
      | client_id     | ${CTX:clientId}     |
      | client_secret | ${CTX:clientSecret} |
      | scope         | ap:project:read     |
    Then the response status code should be 401
    And the JSON response field "code" should be "UNAUTHORIZED"

  @cap:service-accounts @feat:signing-keys
  Scenario: The signing keys are public and cacheable, and they name the key that signs a token
    Given I generate a unique resource name from "sa-keys" and store it as "saName"
    And I create a service account via the control plane from "resources/templates/service-account.yaml" with values:
      | displayName | ${CTX:saName}          |
      | roles       | ["ap_service_account"] |
    And the response status code should be 201
    And I store the JSON response field "serviceAccount.id" as "saId"
    And I store the JSON response field "clientId" as "clientId"
    And I store the JSON response field "clientSecret" as "clientSecret"
    When I send a "GET" request to the control plane root at "/.well-known/jwks"
    Then the response status code should be 200
    And the response header "Content-Type" should be "application/jwk-set+json"
    And the response header "Cache-Control" should be "public, max-age=300"
    And the response header "ETag" should exist
    And the JSON response array field "keys" should have 1 item
    And the JSON response field "keys[0].kty" should be "RSA"
    And the JSON response field "keys[0].alg" should be "RS256"
    And the JSON response field "keys[0].use" should be "sig"
    And the JSON response field "keys[0].d" should not exist
    And I store the JSON response field "keys[0].kid" as "jwksKid"
    And I store the response header "ETag" as "jwksETag"
    When I send a "GET" request to the control plane root at "/.well-known/jwks" with header "If-None-Match" set to "${CTX:jwksETag}"
    Then the response status code should be 304
    And the response body should be empty
    And the response header "ETag" should be "${CTX:jwksETag}"
    When I exchange service account credentials with values:
      | client_id     | ${CTX:clientId}     |
      | client_secret | ${CTX:clientSecret} |
      | scope         | ap:project:read     |
    Then the response status code should be 200
    And the JWT in the JSON response field "access_token" should have header "kid" equal to "${CTX:jwksKid}"
