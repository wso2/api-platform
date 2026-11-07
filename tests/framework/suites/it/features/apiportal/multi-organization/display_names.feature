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

Feature: API Portal shows users by display name in audit fields

  # Each scenario signs in as its own IdP user: a fresh UUID as the sub, with or without a name
  # claim. Audit fields show the stored display name, else the sub; a name is taken only from a
  # verified token.

  Scenario: A user whose token carries a name is shown by that name
    Given I generate a UUID and store it as "userSub"
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub    | ${CTX:userSub} |
      | org_id | default        |
      | roles  | ["ap_admin"]   |
      | name   | Ada Lovelace   |
    And a unique API Portal application is created in portal "api-portal-multi-organization" with token "token" and stored as "appId"
    When I send an API Portal "GET" request to "/applications/${CTX:appId}" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 200
    And the JSON response field "createdBy" should be "Ada Lovelace"
    And the JSON response field "updatedBy" should be "Ada Lovelace"
    When I send an API Portal "GET" request to "/applications" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 200
    And the JSON response array "list" item with "id" equal to "${CTX:appId}" should have "createdBy" equal to "Ada Lovelace"

  Scenario: A user whose token carries no name is shown by their subject
    Given I generate a UUID and store it as "userSub"
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub    | ${CTX:userSub} |
      | org_id | default        |
      | roles  | ["ap_admin"]   |
    And a unique API Portal application is created in portal "api-portal-multi-organization" with token "token" and stored as "appId"
    When I send an API Portal "GET" request to "/applications/${CTX:appId}" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 200
    And the JSON response field "createdBy" should be "${CTX:userSub}"
    And the JSON response field "updatedBy" should be "${CTX:userSub}"

  Scenario: A later token without a name keeps the stored name
    Given I generate a UUID and store it as "userSub"
    And I mint an API Portal IDP token stored as "namedToken" with claims:
      | sub    | ${CTX:userSub} |
      | org_id | default        |
      | roles  | ["ap_admin"]   |
      | name   | Ada Lovelace   |
    And I mint an API Portal IDP token stored as "unnamedToken" with claims:
      | sub    | ${CTX:userSub} |
      | org_id | default        |
      | roles  | ["ap_admin"]   |
    And a unique API Portal application is created in portal "api-portal-multi-organization" with token "namedToken" and stored as "appId"
    When I send an API Portal "PUT" request to "/applications/${CTX:appId}" using portal "api-portal-multi-organization" with token "unnamedToken" and JSON body:
      """
      {"id": "${CTX:appId}", "displayName": "${CTX:appId}", "description": "updated"}
      """
    Then the response status code should be 200
    When I send an API Portal "GET" request to "/applications/${CTX:appId}" using portal "api-portal-multi-organization" with token "unnamedToken"
    Then the response status code should be 200
    And the JSON response field "createdBy" should be "Ada Lovelace"
    And the JSON response field "updatedBy" should be "Ada Lovelace"

  Scenario: A name changed without re-signing the token is rejected and never stored
    Given I generate a UUID and store it as "userSub"
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub    | ${CTX:userSub} |
      | org_id | default        |
      | roles  | ["ap_admin"]   |
      | name   | Ada Lovelace   |
    And I mint an API Portal IDP token stored as "readToken" with claims:
      | sub    | ${CTX:userSub} |
      | org_id | default        |
      | roles  | ["ap_admin"]   |
    And a unique API Portal application is created in portal "api-portal-multi-organization" with token "token" and stored as "appId"
    And I replace claim "name" of API Portal token "token" with "Mallory"
    When I send an API Portal "GET" request to "/applications/${CTX:appId}" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 401
    When I send an API Portal "GET" request to "/applications/${CTX:appId}" using portal "api-portal-multi-organization" with token "readToken"
    Then the response status code should be 200
    And the JSON response field "createdBy" should be "Ada Lovelace"
    And the JSON response field "updatedBy" should be "Ada Lovelace"
