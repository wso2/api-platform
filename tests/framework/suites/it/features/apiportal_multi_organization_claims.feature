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

Feature: API Portal multi-organization organization claims

  # The portal serves every organization under its portal_id and provisions the
  # organization an unknown org claim names.

  Scenario: The configured organization is served as always
    Given I mint an API Portal IDP token stored as "token" with claims:
      | sub    | alice        |
      | org_id | default      |
      | roles  | ["ap_admin"] |
    When I send an API Portal "GET" request to "/organizations/default" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 200
    And the JSON response field "id" should be "default"

  Scenario Outline: A claim differing only in case or trailing spaces does not match an organization
    # SQL Server's default collation would match these to "default" in the query itself.
    Given I mint an API Portal IDP token stored as "token" with claims:
      | sub    | c            |
      | org_id | <claim>      |
      | roles  | ["ap_admin"] |
    When I send an API Portal "GET" request to "/organizations/default" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 403

    Examples:
      | claim      |
      | DEFAULT    |
      | Default    |
      | "default " |

  Scenario: An unknown organization is provisioned from the claim and named from the org-name claim
    Given I generate a unique resource name from "globex" and store it as "globex"
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub      | gina                 |
      | org_id   | ${CTX:globex}        |
      | org_name | Globex ${CTX:globex} |
      | roles    | ["ap_admin"]         |
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 200
    And the API Portal "api-portal-multi-organization" organization with IDP reference "${CTX:globex}" should have handle "globex-${CTX:globex}" and display name "Globex ${CTX:globex}"
    When I send an API Portal "GET" request to "/views" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 200
    And the response body should match pattern ".id.:\s*.default."
    When I send an API Portal "GET" request to "/organizations/globex-${CTX:globex}" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 200
    And the JSON response field "idpRefId" should be "${CTX:globex}"

  Scenario: The org-handle claim becomes the provisioned organization's handle as it is
    Given I generate a unique resource name from "handled" and store it as "handled"
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub        | hana                      |
      | org_id     | ${CTX:handled}            |
      | org_name   | Handled Org ${CTX:handled} |
      | org_handle | HND-${CTX:handled}         |
      | roles      | ["ap_admin"]              |
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 200
    And the API Portal "api-portal-multi-organization" organization with IDP reference "${CTX:handled}" should have handle "hnd-${CTX:handled}" and display name "Handled Org ${CTX:handled}"

  Scenario: A token issued to another application is rejected
    Given I mint an API Portal IDP token stored as "token" with claims:
      | sub    | aud            |
      | org_id | default        |
      | aud    | some-other-app |
      | roles  | ["ap_admin"]   |
    When I send an API Portal "GET" request to "/organizations/default" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 401

  Scenario: A credential with no organization claim falls back to the configured organization
    Given I mint an API Portal IDP token stored as "token" with claims:
      | sub   | nora         |
      | roles | ["ap_admin"] |
    When I send an API Portal "GET" request to "/organizations/default" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 200

  Scenario: An org-handle claim is not taken for the organization claim
    Given I generate a unique resource name from "byhandle" and store it as "byhandle"
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub        | hal              |
      | org_handle | ${CTX:byhandle}  |
      | roles      | ["ap_admin"]     |
    When I send an API Portal "GET" request to "/organizations/default" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 200
    And the API Portal "api-portal-multi-organization" should have 0 organizations with IDP reference "${CTX:byhandle}"

  Scenario: A single-entry list claim is that organization, and one naming two is refused
    Given I generate a unique resource name from "listed" and store it as "listed"
    And I mint an API Portal IDP token stored as "plain" with claims:
      | sub    | l1             |
      | org_id | ${CTX:listed}  |
      | roles  | ["ap_admin"]   |
    And I mint an API Portal IDP token stored as "single" with claims:
      | sub    | l2                 |
      | org_id | ["${CTX:listed}"]  |
      | roles  | ["ap_admin"]       |
    And I mint an API Portal IDP token stored as "double" with claims:
      | sub    | l3                            |
      | org_id | ["${CTX:listed}", "default"]  |
      | roles  | ["ap_admin"]                  |
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "plain"
    Then the response status code should be 200
    When I send an API Portal "GET" request to "/organizations/${CTX:listed}" using portal "api-portal-multi-organization" with token "single"
    Then the response status code should be 200
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "double"
    Then the response status code should be 403

  Scenario: Concurrent first use of a new organization provisions it exactly once
    Given I generate a unique resource name from "race" and store it as "race"
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub      | racer             |
      | org_id   | ${CTX:race}       |
      | org_name | Race ${CTX:race}  |
      | roles    | ["ap_admin"]      |
    Then 10 concurrent API Portal "GET" requests to "/apis" using portal "api-portal-multi-organization" with token "token" should all return status 200
    And the API Portal "api-portal-multi-organization" should have 1 organization with IDP reference "${CTX:race}"

  # Per-organization issuers: the testbench signs a token issued as ".../o/<org>/oauth2/token"
  # with that organization's own key, served at ".../o/<org>/oauth2/jwks".

  Scenario: A token from an organization's own issuer is verified with that organization's keys
    Given I generate a unique resource name from "suborg" and store it as "suborg"
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub        | s1                                                   |
      | org_id     | ${CTX:suborg}                                        |
      | org_name   | Suborg ${CTX:suborg}                                 |
      | org_handle | ${CTX:suborg}                                        |
      | iss        | https://testbench:3015/o/${CTX:suborg}/oauth2/token  |
      | roles      | ["ap_admin"]                                         |
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 200
    And the API Portal "api-portal-multi-organization" should have 1 organizations with IDP reference "${CTX:suborg}"

  Scenario: The template organization claim, not the organization claim, must name the issuer's organization
    # org_id names the issuer's organization, but org_handle, the binding claim, doesn't.
    Given I generate a unique resource name from "unbound" and store it as "unbound"
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub        | s2                                                    |
      | org_id     | ${CTX:unbound}                                        |
      | org_handle | elsewhere-${CTX:unbound}                              |
      | iss        | https://testbench:3015/o/${CTX:unbound}/oauth2/token  |
      | roles      | ["ap_admin"]                                          |
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 401
    And the API Portal "api-portal-multi-organization" should have 0 organizations with IDP reference "${CTX:unbound}"

  Scenario: A per-organization token without the organization claim is rejected, not admitted to the configured organization
    # Bound through org_handle, but no org_id: one organization's issuer must not vouch
    # for a credential that would fall back to the configured organization.
    Given I generate a unique resource name from "noclaim" and store it as "noclaim"
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub        | s3                                                    |
      | org_handle | ${CTX:noclaim}                                        |
      | iss        | https://testbench:3015/o/${CTX:noclaim}/oauth2/token  |
      | roles      | ["ap_admin"]                                          |
    When I send an API Portal "GET" request to "/organizations/default" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 401

  Scenario: A token whose issuer names an organization whose keys didn't sign it is rejected
    # Signed by "signer"'s key, but claiming to be issued by, and for, "forged": the portal
    # fetches "forged"'s keys, which don't verify it.
    Given I generate a unique resource name from "signer" and store it as "signer"
    And I generate a unique resource name from "forged" and store it as "forged"
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub        | s4                                                   |
      | org_id     | ${CTX:forged}                                        |
      | org_handle | ${CTX:forged}                                        |
      | iss        | https://testbench:3015/o/${CTX:signer}/oauth2/token  |
      | roles      | ["ap_admin"]                                         |
    And I replace the issuer of API Portal token "token" with "https://testbench:3015/o/${CTX:forged}/oauth2/token"
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 401
    And the API Portal "api-portal-multi-organization" should have 0 organizations with IDP reference "${CTX:forged}"

  Scenario Outline: A token from an issuer that fits neither the configured issuer nor the template is rejected
    Given I generate a unique resource name from "stray" and store it as "stray"
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub        | s5             |
      | org_id     | ${CTX:stray}   |
      | org_handle | ${CTX:stray}   |
      | iss        | <issuer>       |
      | roles      | ["ap_admin"]   |
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 401
    And the API Portal "api-portal-multi-organization" should have 0 organizations with IDP reference "${CTX:stray}"

    Examples:
      | issuer                                                    |
      | https://testbench:3015/t/${CTX:stray}/oauth2/token        |
      | https://testbench:3015/o/${CTX:stray}.x/oauth2/token      |
