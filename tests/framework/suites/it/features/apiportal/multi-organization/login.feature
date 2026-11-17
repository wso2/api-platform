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

@APIPORTAL
Feature: API Portal multi-organization pages and browser sign-in

  # Every organization's public pages are open, sign-in goes through the one configured
  # callback under the configured organization, and silent sign-in only succeeds for the
  # organization being browsed.

  @APIP-034
  Scenario: Users sign in through the IDP, explicitly and silently, into their own organization
    # A new organization's user signs in from the configured organization's page and lands in their own
    Given I generate a unique resource name from "initech" and store it as "initech"
    And the API Portal browser "ivy" has an IDP session with claims:
      | sub      | ivy                    |
      | org_id   | ${CTX:initech}         |
      | org_name | Initech ${CTX:initech} |
      | roles    | ["ap_admin"]           |
    When I sign in to API Portal "api-portal-multi-organization" from "/api-portal/default/views/default/login" as browser "ivy"
    Then the response status code should be 302
    And the response header "Location" should be "/api-portal/initech-${CTX:initech}/views/default"
    And the API Portal browser "ivy" last IDP authorization parameter "org" should be absent
    When I send an API Portal "GET" request to "/api-portal/initech-${CTX:initech}/views/default/applications" using portal "api-portal-multi-organization" as browser "ivy"
    Then the response status code should be 200
    When I send an API Portal "GET" request to "/api-portal/default/views/default/applications" using portal "api-portal-multi-organization" as browser "ivy"
    Then the response status code should be 403
    When I send an API Portal "GET" request to "/organizations/initech-${CTX:initech}" using portal "api-portal-multi-organization" as browser "ivy"
    Then the response status code should be 200

    # An administrator sees Settings only on their own organization's pages
    Given I generate a unique resource name from "navs" and store it as "navs"
    And the API Portal browser "ada" has an IDP session with claims:
      | sub      | ada              |
      | org_id   | ${CTX:navs}      |
      | org_name | Navs ${CTX:navs} |
      | roles    | ["ap_admin"]     |
    And I sign in to API Portal "api-portal-multi-organization" from "/api-portal/default/views/default/login" as browser "ada"
    And the response status code should be 302
    When I send an API Portal "GET" request to "/api-portal/navs-${CTX:navs}/views/default" using portal "api-portal-multi-organization" as browser "ada"
    Then the response status code should be 200
    And the response body should match pattern "id=.admin-settings."
    And the response body should match pattern "id=.applications."
    When I send an API Portal "GET" request to "/api-portal/default/views/default" using portal "api-portal-multi-organization" as browser "ada"
    Then the response status code should be 200
    And the response body should not contain "admin-settings"
    And the response body should match pattern "id=.applications."
    When I send an API Portal "GET" request to "/api-portal/default/views/default/applications" using portal "api-portal-multi-organization" as browser "ada"
    Then the response status code should be 403
    And the response body should not contain "admin-settings"

    # Another organization's login page sends its hint and an explicit org parameter overrides it
    Given I generate a unique resource name from "hinted" and store it as "hinted"
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub      | h1                   |
      | org_id   | ${CTX:hinted}        |
      | org_name | Hinted ${CTX:hinted} |
      | roles    | ["ap_admin"]         |
    And I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "token"
    And the response status code should be 200
    When I send an unauthenticated API Portal "GET" request to "/api-portal/hinted-${CTX:hinted}/views/default/login" using portal "api-portal-multi-organization"
    Then the response status code should be 302
    And the API Portal redirect parameter "org" should be "${CTX:hinted}"
    When I send an unauthenticated API Portal "GET" request to "/api-portal/default/views/default/login?org=${CTX:hinted}" using portal "api-portal-multi-organization"
    Then the response status code should be 302
    And the API Portal redirect parameter "org" should be "${CTX:hinted}"
    When I send an unauthenticated API Portal "GET" request to "/api-portal/default/views/default/login" using portal "api-portal-multi-organization"
    Then the response status code should be 302
    And the API Portal redirect parameter "org" should be absent

    # A login with no organization claim is recorded as the configured organization
    Given I generate a unique resource name from "elsewhere" and store it as "elsewhere"
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub      | e                          |
      | org_id   | ${CTX:elsewhere}           |
      | org_name | Elsewhere ${CTX:elsewhere} |
      | roles    | ["ap_admin"]               |
    And I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "token"
    And the response status code should be 200
    And the API Portal browser "nobody" has an IDP session with claims:
      | sub   | nobody       |
      | roles | ["ap_admin"] |
    When I sign in to API Portal "api-portal-multi-organization" from "/api-portal/default/views/default/login" as browser "nobody"
    Then the response status code should be 302
    When I send an API Portal "GET" request to "/api-portal/default/views/default/applications" using portal "api-portal-multi-organization" as browser "nobody"
    Then the response status code should be 200
    When I send an API Portal "GET" request to "/api-portal/elsewhere-${CTX:elsewhere}/views/default/applications" using portal "api-portal-multi-organization" as browser "nobody"
    Then the response status code should be 403

    # A login whose organization claim names two organizations is refused and provisions nothing
    Given I generate a unique resource name from "twofold" and store it as "twofold"
    And the API Portal browser "tess" has an IDP session with claims:
      | sub    | tess                          |
      | org_id | ["${CTX:twofold}", "default"] |
      | roles  | ["ap_admin"]                  |
    When I sign in to API Portal "api-portal-multi-organization" from "/api-portal/default/views/default/login" as browser "tess"
    Then the response status code should be 403
    And the API Portal "api-portal-multi-organization" should have 0 organizations with IDP reference "${CTX:twofold}"

    # Silent sign-in asks the IDP only for the organization being browsed
    Given I generate a unique resource name from "silent-x" and store it as "silentX"
    And I generate a unique resource name from "silent-y" and store it as "silentY"
    And I mint an API Portal IDP token stored as "tokenY" with claims:
      | sub      | s-y                     |
      | org_id   | ${CTX:silentY}          |
      | org_name | Silent Y ${CTX:silentY} |
      | roles    | ["ap_admin"]            |
    And I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "tokenY"
    And the response status code should be 200
    And the API Portal browser "sam" has an IDP session with claims:
      | sub      | sam                     |
      | org_id   | ${CTX:silentX}          |
      | org_name | Silent X ${CTX:silentX} |
      | roles    | ["ap_admin"]            |
    When I browse API Portal page "/api-portal/silent-y-${CTX:silentY}/views/default" using portal "api-portal-multi-organization" as browser "sam"
    Then the response status code should be 200
    And the API Portal browser "sam" should be on page "/api-portal/silent-y-${CTX:silentY}/views/default"
    And the API Portal browser "sam" last IDP authorization parameter "prompt" should be "none"
    And the API Portal browser "sam" last IDP authorization parameter "org" should be "${CTX:silentY}"
    When I send an API Portal "GET" request to "/organizations/silent-x-${CTX:silentX}" using portal "api-portal-multi-organization" as browser "sam"
    Then the response status code should be 401

    # Silent sign-in signs the visitor in on their own organization's page, in place
    Given I generate a unique resource name from "silent-x" and store it as "silentX"
    And I mint an API Portal IDP token stored as "tokenX" with claims:
      | sub      | s-x                     |
      | org_id   | ${CTX:silentX}          |
      | org_name | Silent X ${CTX:silentX} |
      | roles    | ["ap_admin"]            |
    And I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "tokenX"
    And the response status code should be 200
    And the API Portal browser "sam2" has an IDP session with claims:
      | sub      | sam                     |
      | org_id   | ${CTX:silentX}          |
      | org_name | Silent X ${CTX:silentX} |
      | roles    | ["ap_admin"]            |
    When I browse API Portal page "/api-portal/silent-x-${CTX:silentX}/views/default" using portal "api-portal-multi-organization" as browser "sam2"
    Then the response status code should be 200
    And the API Portal browser "sam2" should be on page "/api-portal/silent-x-${CTX:silentX}/views/default"
    When I send an API Portal "GET" request to "/organizations/silent-x-${CTX:silentX}" using portal "api-portal-multi-organization" as browser "sam2"
    Then the response status code should be 200

    # Silent sign-in from the configured organization's page signs in whoever the IDP knows without moving them
    Given I generate a unique resource name from "silent-x" and store it as "silentX"
    And the API Portal browser "sam3" has an IDP session with claims:
      | sub      | sam                     |
      | org_id   | ${CTX:silentX}          |
      | org_name | Silent X ${CTX:silentX} |
      | roles    | ["ap_admin"]            |
    When I browse API Portal page "/api-portal/default/views/default" using portal "api-portal-multi-organization" as browser "sam3"
    Then the response status code should be 200
    And the API Portal browser "sam3" should be on page "/api-portal/default/views/default"
    And the API Portal browser "sam3" last IDP authorization parameter "org" should be absent
    When I send an API Portal "GET" request to "/organizations/silent-x-${CTX:silentX}" using portal "api-portal-multi-organization" as browser "sam3"
    Then the response status code should be 200

    # An explicit login after a failed silent attempt still lands in the user's own organization
    Given I generate a unique resource name from "silent-x" and store it as "silentX"
    And the API Portal browser "sam4" has no IDP session
    And I browse API Portal page "/api-portal/default/views/default" using portal "api-portal-multi-organization" as browser "sam4"
    And the response status code should be 200
    And the API Portal browser "sam4" has an IDP session with claims:
      | sub      | sam                     |
      | org_id   | ${CTX:silentX}          |
      | org_name | Silent X ${CTX:silentX} |
      | roles    | ["ap_admin"]            |
    When I sign in to API Portal "api-portal-multi-organization" from "/api-portal/default/views/default/login" as browser "sam4"
    Then the response status code should be 302
    And the response header "Location" should be "/api-portal/silent-x-${CTX:silentX}/views/default"

    # After sign-in a user returns to a page of their own organization and otherwise lands on its default view
    Given I generate a unique resource name from "landing" and store it as "landing"
    And I generate a unique resource name from "foreign" and store it as "foreign"
    And I mint an API Portal IDP token stored as "tokenLanding" with claims:
      | sub      | l1                     |
      | org_id   | ${CTX:landing}         |
      | org_name | Landing ${CTX:landing} |
      | roles    | ["ap_admin"]           |
    And I mint an API Portal IDP token stored as "tokenForeign" with claims:
      | sub      | f1                     |
      | org_id   | ${CTX:foreign}         |
      | org_name | Foreign ${CTX:foreign} |
      | roles    | ["ap_admin"]           |
    And I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "tokenLanding"
    And the response status code should be 200
    And I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "tokenForeign"
    And the response status code should be 200
    And the API Portal browser "own" has an IDP session with claims:
      | sub      | lara                   |
      | org_id   | ${CTX:landing}         |
      | org_name | Landing ${CTX:landing} |
      | roles    | ["ap_admin"]           |
    And the API Portal browser "visiting" has an IDP session with claims:
      | sub      | lara                   |
      | org_id   | ${CTX:landing}         |
      | org_name | Landing ${CTX:landing} |
      | roles    | ["ap_admin"]           |
    When I browse API Portal page "/api-portal/landing-${CTX:landing}/views/default/applications" using portal "api-portal-multi-organization" as browser "own"
    Then the response status code should be 200
    And the API Portal browser "own" should be on page "/api-portal/landing-${CTX:landing}/views/default/applications"
    When I browse API Portal page "/api-portal/foreign-${CTX:foreign}/views/default/applications" using portal "api-portal-multi-organization" as browser "visiting"
    Then the response status code should be 200
    And the API Portal browser "visiting" should be on page "/api-portal/landing-${CTX:landing}/views/default"

  @APIP-034
  Scenario: A user's display name comes from their token and is kept when a later token has none
    # A user whose token carries a name is shown by that name
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

    # A user whose token carries no name is shown by their subject
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

    # A later token without a name keeps the stored name
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

    # A name changed without re-signing the token is rejected and never stored
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
