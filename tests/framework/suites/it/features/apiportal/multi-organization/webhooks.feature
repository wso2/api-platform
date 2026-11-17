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
Feature: API Portal multi-organization webhook delivery

  # The dispatcher and delivery worker of a multi-organization portal handle every organization
  # under its portal_id; each organization's subscribers still receive only its own events.

  @APIP-055
  Scenario: Webhooks and rows stay within their organization and portal_id
    # Each organization's subscribers receive exactly that organization's events, once
    Given I generate a unique resource name from "hook-a" and store it as "hookA"
    And I generate a unique resource name from "hook-b" and store it as "hookB"
    And I mint an API Portal IDP token stored as "tokenA" with claims:
      | sub      | wh-a                |
      | org_id   | ${CTX:hookA}        |
      | org_name | Hook ${CTX:hookA}   |
      | roles    | ["ap_admin"]        |
    And I mint an API Portal IDP token stored as "tokenB" with claims:
      | sub      | wh-b                |
      | org_id   | ${CTX:hookB}        |
      | org_name | Hook ${CTX:hookB}   |
      | roles    | ["ap_admin"]        |
    And an API Portal webhook subscriber for events "apikey.*" delivering to sink "a" is registered in portal "api-portal-multi-organization" with token "tokenA"
    And an API Portal webhook subscriber for events "apikey.*" delivering to sink "b" is registered in portal "api-portal-multi-organization" with token "tokenB"
    And a unique API Portal REST API is created in portal "api-portal-multi-organization" with token "tokenA" and stored as "apiA"
    And a unique API Portal REST API is created in portal "api-portal-multi-organization" with token "tokenB" and stored as "apiB"
    And I store the API Portal "api-portal-multi-organization" organization id for IDP reference "${CTX:hookA}" as "orgA"
    And I store the API Portal "api-portal-multi-organization" organization id for IDP reference "${CTX:hookB}" as "orgB"
    When I generate 2 API Portal API keys for API "apiA" in portal "api-portal-multi-organization" with token "tokenA"
    And I generate 2 API Portal API keys for API "apiB" in portal "api-portal-multi-organization" with token "tokenB"
    Then the API Portal webhook sink "a" should receive exactly 2 "apikey.generated" events for organization "${CTX:orgA}"
    And the API Portal webhook sink "b" should receive exactly 2 "apikey.generated" events for organization "${CTX:orgB}"

    # Another portal's organization with the same IDP reference is neither used nor in the way
    Given I generate a unique resource name from "shared" and store it as "shared"
    And API Portal rows for a portal_id no portal serves, with an organization whose IDP reference is "${CTX:shared}", are seeded in the database of "api-portal-multi-organization" and stored as "unowned"
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub        | tia           |
      | org_id     | ${CTX:shared} |
      | org_name   | ${CTX:shared} |
      | org_handle | ${CTX:shared} |
      | roles      | ["ap_admin"]  |
    # The seeded organization has the same idp_ref_id, handle and display name the token's
    # claims provision, so a lookup that reached across portals would resolve to it, or
    # disambiguate the new organization's names against it.
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 200
    And the API Portal "api-portal-multi-organization" organization with IDP reference "${CTX:shared}" should have handle "${CTX:shared}" and display name "${CTX:shared}"
    When I send an API Portal "GET" request to "/organizations/${CTX:shared}" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 200
    And the JSON response field "idpRefId" should be "${CTX:shared}"

    # Events and deliveries under a portal_id no portal serves are left alone
    Given I generate a unique resource name from "shared" and store it as "shared"
    And API Portal rows for a portal_id no portal serves, with an organization whose IDP reference is "${CTX:shared}", are seeded in the database of "api-portal-multi-organization" and stored as "unowned"
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub        | tia           |
      | org_id     | ${CTX:shared} |
      | org_name   | ${CTX:shared} |
      | org_handle | ${CTX:shared} |
      | roles      | ["ap_admin"]  |
    # The portal's own event is delivered after the rows are seeded, which proves its
    # dispatcher and delivery worker have both run since.
    Given an API Portal webhook subscriber for events "apikey.*" delivering to sink "proof" is registered in portal "api-portal-multi-organization" with token "token"
    And a unique API Portal REST API is created in portal "api-portal-multi-organization" with token "token" and stored as "api"
    And I store the API Portal "api-portal-multi-organization" organization id for IDP reference "${CTX:shared}" as "org"
    When I generate 1 API Portal API keys for API "api" in portal "api-portal-multi-organization" with token "token"
    Then the API Portal webhook sink "proof" should receive exactly 1 "apikey.generated" events for organization "${CTX:org}"
    And the seeded API Portal rows "unowned" should stay untouched
