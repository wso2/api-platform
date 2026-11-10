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
Feature: API Portal multi-organization organization claims

  # The portal serves every organization under its portal_id and provisions the
  # organization an unknown org claim names.

  @APIP-030
  Scenario: Organization claims map callers to organizations and provision a new one exactly once
    # The configured organization is served as always
    Given I mint an API Portal IDP token stored as "token" with claims:
      | sub    | alice        |
      | org_id | default      |
      | roles  | ["ap_admin"] |
    When I send an API Portal "GET" request to "/organizations/default" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 200
    And the JSON response field "id" should be "default"

    # A claim differing only in case or trailing spaces does not match an organization
    # SQL Server's default collation would match these to "default" in the query itself.
    Given I mint an API Portal IDP token stored as "token" with claims:
      | sub    | c            |
      | org_id | DEFAULT      |
      | roles  | ["ap_admin"] |
    When I send an API Portal "GET" request to "/organizations/default" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 403

    # A claim differing only in case or trailing spaces does not match an organization
    # SQL Server's default collation would match these to "default" in the query itself.
    Given I mint an API Portal IDP token stored as "token" with claims:
      | sub    | c            |
      | org_id | Default      |
      | roles  | ["ap_admin"] |
    When I send an API Portal "GET" request to "/organizations/default" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 403

    # A claim differing only in case or trailing spaces does not match an organization
    # SQL Server's default collation would match these to "default" in the query itself.
    Given I mint an API Portal IDP token stored as "token" with claims:
      | sub    | c            |
      | org_id | "default "      |
      | roles  | ["ap_admin"] |
    When I send an API Portal "GET" request to "/organizations/default" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 403

    # A token issued to another application is rejected
    Given I mint an API Portal IDP token stored as "token" with claims:
      | sub    | aud            |
      | org_id | default        |
      | aud    | some-other-app |
      | roles  | ["ap_admin"]   |
    When I send an API Portal "GET" request to "/organizations/default" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 401

    # A credential with no organization claim falls back to the configured organization
    Given I mint an API Portal IDP token stored as "token" with claims:
      | sub   | nora         |
      | roles | ["ap_admin"] |
    When I send an API Portal "GET" request to "/organizations/default" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 200

    # An org-handle claim is not taken for the organization claim
    Given I generate a unique resource name from "byhandle" and store it as "byhandle"
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub        | hal              |
      | org_handle | ${CTX:byhandle}  |
      | roles      | ["ap_admin"]     |
    When I send an API Portal "GET" request to "/organizations/default" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 200
    And the API Portal "api-portal-multi-organization" should have 0 organizations with IDP reference "${CTX:byhandle}"

    # An unknown organization is provisioned from the claim and named from the org-name claim
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
    When I send an API Portal "GET" request to "/labels" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 200
    And the response body should match pattern ".id.:\s*.default."
    When I send an API Portal "GET" request to "/subscription-plans" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 200
    And the response body should contain "Bronze"
    When I send an API Portal "GET" request to "/organizations/globex-${CTX:globex}" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 200
    And the JSON response field "idpRefId" should be "${CTX:globex}"

    # The org-handle claim becomes the provisioned organization's handle as it is
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

    # A single-entry list claim is that organization, and one naming two is refused
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

    # Concurrent first use of a new organization provisions it exactly once
    Given I generate a unique resource name from "race" and store it as "race"
    And the API Portal "api-portal-multi-organization" organization with IDP reference "${CTX:race}" is removed after the scenario
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub      | racer             |
      | org_id   | ${CTX:race}       |
      | org_name | Race ${CTX:race}  |
      | roles    | ["ap_admin"]      |
    Then 10 concurrent API Portal "GET" requests to "/apis" using portal "api-portal-multi-organization" with token "token" should all return status 200
    And the API Portal "api-portal-multi-organization" should have 1 organization with IDP reference "${CTX:race}"

    # A later token with new organization names leaves the provisioned names unchanged
    Given I generate a unique resource name from "renamed" and store it as "renamed"
    And the API Portal "api-portal-multi-organization" organization with IDP reference "${CTX:renamed}" is removed after the scenario
    And I mint an API Portal IDP token stored as "first" with claims:
      | sub        | r1                    |
      | org_id     | ${CTX:renamed}        |
      | org_name   | First ${CTX:renamed}  |
      | org_handle | first-${CTX:renamed}  |
      | roles      | ["ap_admin"]          |
    And I mint an API Portal IDP token stored as "later" with claims:
      | sub        | r2                    |
      | org_id     | ${CTX:renamed}        |
      | org_name   | Second ${CTX:renamed} |
      | org_handle | second-${CTX:renamed} |
      | roles      | ["ap_admin"]          |
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "first"
    Then the response status code should be 200
    And the API Portal "api-portal-multi-organization" organization with IDP reference "${CTX:renamed}" should have handle "first-${CTX:renamed}" and display name "First ${CTX:renamed}"
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "later"
    Then the response status code should be 200
    And the API Portal "api-portal-multi-organization" should have 1 organization with IDP reference "${CTX:renamed}"
    And the API Portal "api-portal-multi-organization" organization with IDP reference "${CTX:renamed}" should have handle "first-${CTX:renamed}" and display name "First ${CTX:renamed}"
    When I send an API Portal "GET" request to "/organizations/first-${CTX:renamed}" using portal "api-portal-multi-organization" with token "later"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "First ${CTX:renamed}"
    When I send an API Portal "GET" request to "/organizations/second-${CTX:renamed}" using portal "api-portal-multi-organization" with token "later"
    Then the response status code should be 403

    # A claim is matched on idp_ref_id only, never on a handle
    Given I generate a unique resource name from "iso-a" and store it as "isoA"
    And I generate a unique resource name from "iso-b" and store it as "isoB"
    And I mint an API Portal IDP token stored as "tokenA" with claims:
      | sub      | ann                  |
      | org_id   | ${CTX:isoA}          |
      | org_name | Org A ${CTX:isoA}    |
      | roles    | ["ap_admin"]         |
    And I mint an API Portal IDP token stored as "tokenB" with claims:
      | sub      | ben                  |
      | org_id   | ${CTX:isoB}          |
      | org_name | Org B ${CTX:isoB}    |
      | roles    | ["ap_admin"]         |
    Given I mint an API Portal IDP token stored as "tokenHandle" with claims:
      | sub    | h                     |
      | org_id | org-a-${CTX:isoA}     |
      | roles  | ["ap_admin"]          |
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "tokenA"
    Then the response status code should be 200
    When I send an API Portal "GET" request to "/organizations/org-a-${CTX:isoA}" using portal "api-portal-multi-organization" with token "tokenHandle"
    Then the response status code should be 403
    And the API Portal "api-portal-multi-organization" organization with IDP reference "org-a-${CTX:isoA}" should have a handle matching "org-a-${CTX:isoA}-[0-9a-f]{6}"

  @APIP-054
  Scenario: Per-organization issuers are verified with that organization's own keys and claims
    # A token from an organization's own issuer is verified with that organization's keys
    # Per-organization issuers: the testbench signs a token issued as ".../o/<org>/oauth2/token"
    # with that organization's own key, served at ".../o/<org>/oauth2/jwks".
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

    # The template organization claim, not the organization claim, must name the issuer's organization
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

    # A per-organization token without the organization claim is rejected, not admitted to the configured organization
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

    # A token whose issuer names an organization whose keys didn't sign it is rejected
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
    And I replace claim "iss" of API Portal token "token" with "https://testbench:3015/o/${CTX:forged}/oauth2/token"
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 401
    And the API Portal "api-portal-multi-organization" should have 0 organizations with IDP reference "${CTX:forged}"

    # A token from an issuer that fits neither the configured issuer nor the template is rejected
    Given I generate a unique resource name from "stray" and store it as "stray"
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub        | s5             |
      | org_id     | ${CTX:stray}   |
      | org_handle | ${CTX:stray}   |
      | iss        | https://testbench:3015/t/${CTX:stray}/oauth2/token       |
      | roles      | ["ap_admin"]   |
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 401
    And the API Portal "api-portal-multi-organization" should have 0 organizations with IDP reference "${CTX:stray}"

    # A token from an issuer that fits neither the configured issuer nor the template is rejected
    Given I generate a unique resource name from "stray" and store it as "stray"
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub        | s5             |
      | org_id     | ${CTX:stray}   |
      | org_handle | ${CTX:stray}   |
      | iss        | https://testbench:3015/o/${CTX:stray}.x/oauth2/token       |
      | roles      | ["ap_admin"]   |
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "token"
    Then the response status code should be 401
    And the API Portal "api-portal-multi-organization" should have 0 organizations with IDP reference "${CTX:stray}"
