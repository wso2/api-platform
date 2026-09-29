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

Feature: API Portal multi-tenancy portals sharing one database

  # A second multi-tenancy portal runs on the same database under its own portal_id. Neither
  # portal sees, provisions over, or delivers the other's rows.

  Background:
    Given I generate a unique resource name from "two-portals" and store it as "shared"
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub        | tia                         |
      | org_id     | ${CTX:shared}               |
      | org_name   | Two Portals ${CTX:shared}   |
      | org_handle | two-${CTX:shared}           |
      | roles      | ["ap_admin"]                |

  Scenario: One IDP organization is provisioned separately in each portal and neither sees the other's APIs
    Given a unique API Portal REST API is created in portal "api-portal-multi-tenancy" with token "token" and stored as "here"
    And a unique API Portal REST API is created in portal "api-portal-multi-tenancy-other-portal" with token "token" and stored as "there"
    Then the API Portal "api-portal-multi-tenancy" organization with IDP reference "${CTX:shared}" should have handle "two-${CTX:shared}" and display name "Two Portals ${CTX:shared}"
    And the API Portal "api-portal-multi-tenancy-other-portal" organization with IDP reference "${CTX:shared}" should have handle "two-${CTX:shared}" and display name "Two Portals ${CTX:shared}"
    When I send an API Portal "GET" request to "/apis/${CTX:here}" using portal "api-portal-multi-tenancy" with token "token"
    Then the response status code should be 200
    When I send an API Portal "GET" request to "/apis/${CTX:there}" using portal "api-portal-multi-tenancy" with token "token"
    Then the response status code should be 404
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-tenancy" with token "token"
    Then the response status code should be 200
    And the response body should not contain "${CTX:there}"
    When I send an API Portal "GET" request to "/apis/${CTX:there}" using portal "api-portal-multi-tenancy-other-portal" with token "token"
    Then the response status code should be 200
    When I send an API Portal "GET" request to "/apis/${CTX:here}" using portal "api-portal-multi-tenancy-other-portal" with token "token"
    Then the response status code should be 404

  Scenario: Each portal delivers its own subscribers' events, and only those
    Given an API Portal webhook subscriber for events "apikey.*" delivering to sink "here" is registered in portal "api-portal-multi-tenancy" with token "token"
    And an API Portal webhook subscriber for events "apikey.*" delivering to sink "there" is registered in portal "api-portal-multi-tenancy-other-portal" with token "token"
    And a unique API Portal REST API is created in portal "api-portal-multi-tenancy" with token "token" and stored as "here"
    And a unique API Portal REST API is created in portal "api-portal-multi-tenancy-other-portal" with token "token" and stored as "there"
    And I store the API Portal "api-portal-multi-tenancy" organization id for IDP reference "${CTX:shared}" as "hereOrg"
    And I store the API Portal "api-portal-multi-tenancy-other-portal" organization id for IDP reference "${CTX:shared}" as "thereOrg"
    When I generate 3 API Portal API keys for API "here" in portal "api-portal-multi-tenancy" with token "token"
    And I generate 3 API Portal API keys for API "there" in portal "api-portal-multi-tenancy-other-portal" with token "token"
    Then the API Portal webhook sink "here" should receive exactly 3 "apikey.generated" events for organization "${CTX:hereOrg}"
    And the API Portal webhook sink "there" should receive exactly 3 "apikey.generated" events for organization "${CTX:thereOrg}"

  Scenario: Rows under a portal_id no portal serves are left alone
    # Each portal dispatches its own events the moment they are published, so the scenario
    # above could pass even if a claim crossed portals now and then. Nothing serves these
    # rows, so any change to them is a portal taking another portal's work.
    Given API Portal rows for a portal_id no portal serves are seeded in the database of "api-portal-multi-tenancy" and stored as "unowned"
    And an API Portal webhook subscriber for events "apikey.*" delivering to sink "here" is registered in portal "api-portal-multi-tenancy" with token "token"
    And an API Portal webhook subscriber for events "apikey.*" delivering to sink "there" is registered in portal "api-portal-multi-tenancy-other-portal" with token "token"
    And a unique API Portal REST API is created in portal "api-portal-multi-tenancy" with token "token" and stored as "here"
    And a unique API Portal REST API is created in portal "api-portal-multi-tenancy-other-portal" with token "token" and stored as "there"
    And I store the API Portal "api-portal-multi-tenancy" organization id for IDP reference "${CTX:shared}" as "hereOrg"
    And I store the API Portal "api-portal-multi-tenancy-other-portal" organization id for IDP reference "${CTX:shared}" as "thereOrg"
    When I generate 1 API Portal API keys for API "here" in portal "api-portal-multi-tenancy" with token "token"
    And I generate 1 API Portal API keys for API "there" in portal "api-portal-multi-tenancy-other-portal" with token "token"
    Then the API Portal webhook sink "here" should receive exactly 1 "apikey.generated" events for organization "${CTX:hereOrg}"
    And the API Portal webhook sink "there" should receive exactly 1 "apikey.generated" events for organization "${CTX:thereOrg}"
    And the seeded API Portal rows "unowned" should stay untouched
