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

Feature: API Portal multi-organization on a database shared with other portals

  # Other portals may share the database under their own portal_id. The seeded rows stand in
  # for one: they sit under a portal_id no running portal serves, so this portal must never
  # claim them, resolve a credential to them, or treat their names as taken.

  Background:
    Given I generate a unique resource name from "shared" and store it as "shared"
    And API Portal rows for a portal_id no portal serves, with an organization whose IDP reference is "${CTX:shared}", are seeded in the database of "api-portal-multi-organization" and stored as "unowned"
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub        | tia           |
      | org_id     | ${CTX:shared} |
      | org_name   | ${CTX:shared} |
      | org_handle | ${CTX:shared} |
      | roles      | ["ap_admin"]  |

  Scenario: Another portal's organization with the same IDP reference is neither used nor in the way
    # The seeded organization has the same idp_ref_id, handle and display name the token's
    # claims provision, so a lookup that reached across portals would resolve to it, or
    # disambiguate the new organization's names against it.
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 200
    And the API Portal "api-portal-multi-organization" organization with IDP reference "${CTX:shared}" should have handle "${CTX:shared}" and display name "${CTX:shared}"
    When I send an API Portal "GET" request to "/organizations/${CTX:shared}" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 200
    And the JSON response field "idpRefId" should be "${CTX:shared}"

  Scenario: Events and deliveries under a portal_id no portal serves are left alone
    # The portal's own event is delivered after the rows are seeded, which proves its
    # dispatcher and delivery worker have both run since.
    Given an API Portal webhook subscriber for events "apikey.*" delivering to sink "proof" is registered in portal "api-portal-multi-organization" with token "token"
    And a unique API Portal REST API is created in portal "api-portal-multi-organization" with token "token" and stored as "api"
    And I store the API Portal "api-portal-multi-organization" organization id for IDP reference "${CTX:shared}" as "org"
    When I generate 1 API Portal API keys for API "api" in portal "api-portal-multi-organization" with token "token"
    Then the API Portal webhook sink "proof" should receive exactly 1 "apikey.generated" events for organization "${CTX:org}"
    And the seeded API Portal rows "unowned" should stay untouched
