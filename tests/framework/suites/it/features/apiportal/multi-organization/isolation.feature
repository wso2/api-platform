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
Feature: API Portal multi-organization isolation

  # The org claims deliberately differ from the handles the organization names derive.

  @APIP-030 @type:security
  Scenario: Organizations are isolated from each other, in their data, their pages and when publishing
    # An organization never sees another organization's APIs
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
    Given a unique API Portal REST API is created in portal "api-portal-multi-organization" with token "tokenA" and stored as "api"
    When I send an API Portal "GET" request to "/apis/${CTX:api}" using portal "api-portal-multi-organization" with token "tokenA"
    Then the response status code should be 200
    When I send an API Portal "GET" request to "/apis/${CTX:api}" using portal "api-portal-multi-organization" with token "tokenB"
    Then the response status code should be 404
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "tokenB"
    Then the response status code should be 200
    And the response body should not contain "${CTX:api}"
    When I send an API Portal "POST" request to "/apis/${CTX:api}/api-keys/generate" using portal "api-portal-multi-organization" with token "tokenB" and JSON body:
      """
      {"id": "stolen"}
      """
    Then the response status code should be 404
    Given the API Portal browser "visitor" has no IDP session
    When I browse API Portal page "/api-portal/org-a-${CTX:isoA}/views/default/apis" using portal "api-portal-multi-organization" as browser "visitor"
    Then the response status code should be 200
    And the API Portal browser "visitor" should be on page "/api-portal/org-a-${CTX:isoA}/views/default/apis"
    And the response body should contain "${CTX:api}"
    When I browse API Portal page "/api-portal/org-b-${CTX:isoB}/views/default/apis" using portal "api-portal-multi-organization" as browser "visitor"
    Then the response status code should be 200
    And the API Portal browser "visitor" should be on page "/api-portal/org-b-${CTX:isoB}/views/default/apis"
    And the response body should not contain "${CTX:api}"

    # The organization APIs accept only the caller's own organization
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
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "tokenB"
    Then the response status code should be 200
    When I send an API Portal "GET" request to "/organizations/org-a-${CTX:isoA}" using portal "api-portal-multi-organization" with token "tokenA"
    Then the response status code should be 200
    When I send an API Portal "GET" request to "/organizations/org-b-${CTX:isoB}" using portal "api-portal-multi-organization" with token "tokenA"
    Then the response status code should be 403
    When I send an API Portal "GET" request to "/organizations/default" using portal "api-portal-multi-organization" with token "tokenA"
    Then the response status code should be 403
    When I send an API Portal "PUT" request to "/organizations/default" using portal "api-portal-multi-organization" with token "tokenA" and JSON body:
      """
      {"id": "default", "displayName": "taken over", "idpRefId": "default"}
      """
    Then the response status code should be 403

    # An administrator updates their own organization and no other
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
    Given I mint an API Portal IDP token stored as "tokenSubscriber" with claims:
      | sub    | sue               |
      | org_id | ${CTX:isoA}       |
      | roles  | ["ap_subscriber"] |
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "tokenB"
    Then the response status code should be 200
    When I send an API Portal "PUT" request to "/organizations/org-a-${CTX:isoA}" using portal "api-portal-multi-organization" with token "tokenA" and JSON body:
      """
      {"id": "org-a-${CTX:isoA}", "idpRefId": "${CTX:isoA}", "displayName": "Renamed ${CTX:isoA}"}
      """
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Renamed ${CTX:isoA}"
    When I send an API Portal "GET" request to "/organizations/org-a-${CTX:isoA}" using portal "api-portal-multi-organization" with token "tokenA"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Renamed ${CTX:isoA}"
    When I send an API Portal "GET" request to "/organizations/org-b-${CTX:isoB}" using portal "api-portal-multi-organization" with token "tokenB"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Org B ${CTX:isoB}"
    When I send an API Portal "PUT" request to "/organizations/org-a-${CTX:isoA}" using portal "api-portal-multi-organization" with token "tokenA" and JSON body:
      """
      {"id": "org-a-${CTX:isoA}", "idpRefId": "elsewhere-${CTX:isoA}", "displayName": "Re-pointed ${CTX:isoA}"}
      """
    Then the response status code should be 400
    When I send an API Portal "PUT" request to "/organizations/org-a-${CTX:isoA}" using portal "api-portal-multi-organization" with token "tokenSubscriber" and JSON body:
      """
      {"id": "org-a-${CTX:isoA}", "idpRefId": "${CTX:isoA}", "displayName": "Subscriber ${CTX:isoA}"}
      """
    Then the response status code should be 403
    When I send an API Portal "GET" request to "/organizations/org-a-${CTX:isoA}" using portal "api-portal-multi-organization" with token "tokenA"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Renamed ${CTX:isoA}"
    And the JSON response field "idpRefId" should be "${CTX:isoA}"

    # The MCP registry's write endpoints act only in the caller's own organization
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
    Given a unique API Portal MCP server is created in portal "api-portal-multi-organization" with token "tokenA" and stored as "mcp"
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "tokenB"
    Then the response status code should be 200
    When I send an API Portal "DELETE" request to "/api-portal/registry/org-a-${CTX:isoA}/v0.1/servers/${CTX:mcp}/versions/1.0.0" using portal "api-portal-multi-organization" with token "tokenB"
    Then the response status code should be 403
    When I send an API Portal "PATCH" request to "/api-portal/registry/org-a-${CTX:isoA}/v0.1/servers/${CTX:mcp}/status" using portal "api-portal-multi-organization" with token "tokenB" and JSON body:
      """
      {"status": "deprecated"}
      """
    Then the response status code should be 403
    When I send an unauthenticated API Portal "GET" request to "/api-portal/registry/org-a-${CTX:isoA}/v0.1/servers" using portal "api-portal-multi-organization"
    Then the response status code should be 200
    And the response body should contain "${CTX:mcp}"
    When I send an API Portal "DELETE" request to "/api-portal/registry/org-a-${CTX:isoA}/v0.1/servers/${CTX:mcp}/versions/1.0.0" using portal "api-portal-multi-organization" with token "tokenA"
    Then the response status code should be 200

    # Anyone can browse an existing organization's public pages and an unknown handle is not found
    Given I generate a unique resource name from "pages" and store it as "pages"
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub      | p                  |
      | org_id   | ${CTX:pages}       |
      | org_name | Pages ${CTX:pages} |
      | roles    | ["ap_admin"]       |
    And I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "token"
    And the response status code should be 200
    And the API Portal browser "visitor" has no IDP session
    When I browse API Portal page "/api-portal/pages-${CTX:pages}/views/default" using portal "api-portal-multi-organization" as browser "visitor"
    Then the response status code should be 200
    And the API Portal browser "visitor" should be on page "/api-portal/pages-${CTX:pages}/views/default"
    When I send an unauthenticated API Portal "GET" request to "/api-portal/no-such-${CTX:pages}/views/default" using portal "api-portal-multi-organization"
    Then the response status code should be 404
    And the API Portal "api-portal-multi-organization" should have 0 organizations with IDP reference "no-such-${CTX:pages}"

    # A shared-key call publishes into the configured organization
    Given I generate a unique resource name from "sk-org" and store it as "skOrg"
    And I mint an API Portal IDP token stored as "defaultToken" with claims:
      | sub    | dora         |
      | org_id | default      |
      | roles  | ["ap_admin"] |
    And I mint an API Portal IDP token stored as "skOrgToken" with claims:
      | sub        | sam                       |
      | org_id     | ${CTX:skOrg}              |
      | org_name   | SK Org ${CTX:skOrg}       |
      | org_handle | sk-${CTX:skOrg}           |
      | roles      | ["ap_admin"]              |
    And I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "skOrgToken"
    And the response status code should be 200
    When I publish a unique REST API to API Portal "api-portal-multi-organization" with the shared key and store its id as "api"
    Then the response status code should be 201
    When I send an API Portal "GET" request to "/apis/${CTX:api}" using portal "api-portal-multi-organization" with token "defaultToken"
    Then the response status code should be 200
    When I send an API Portal "GET" request to "/apis/${CTX:api}" using portal "api-portal-multi-organization" with token "skOrgToken"
    Then the response status code should be 404
    When I publish a unique REST API to API Portal "api-portal-multi-organization" with the shared key and header "organization" set to "default" and store its id as "named"
    Then the response status code should be 201
    When I send an API Portal "GET" request to "/apis/${CTX:named}" using portal "api-portal-multi-organization" with token "defaultToken"
    Then the response status code should be 200

    # A shared-key call naming another organization is refused and creates nothing
    Given I generate a unique resource name from "sk-org" and store it as "skOrg"
    And I mint an API Portal IDP token stored as "defaultToken" with claims:
      | sub    | dora         |
      | org_id | default      |
      | roles  | ["ap_admin"] |
    And I mint an API Portal IDP token stored as "skOrgToken" with claims:
      | sub        | sam                       |
      | org_id     | ${CTX:skOrg}              |
      | org_name   | SK Org ${CTX:skOrg}       |
      | org_handle | sk-${CTX:skOrg}           |
      | roles      | ["ap_admin"]              |
    And I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "skOrgToken"
    And the response status code should be 200
    When I publish a unique REST API to API Portal "api-portal-multi-organization" with the shared key and header "organization" set to "sk-${CTX:skOrg}" and store its id as "refused"
    Then the response status code should be 403
    When I send an API Portal "GET" request to "/apis/${CTX:refused}" using portal "api-portal-multi-organization" with token "skOrgToken"
    Then the response status code should be 404
    When I send an API Portal "GET" request to "/apis/${CTX:refused}" using portal "api-portal-multi-organization" with token "defaultToken"
    Then the response status code should be 404

    # A shared-key call naming another organization is refused and creates nothing
    Given I generate a unique resource name from "sk-org" and store it as "skOrg"
    And I mint an API Portal IDP token stored as "defaultToken" with claims:
      | sub    | dora         |
      | org_id | default      |
      | roles  | ["ap_admin"] |
    And I mint an API Portal IDP token stored as "skOrgToken" with claims:
      | sub        | sam                       |
      | org_id     | ${CTX:skOrg}              |
      | org_name   | SK Org ${CTX:skOrg}       |
      | org_handle | sk-${CTX:skOrg}           |
      | roles      | ["ap_admin"]              |
    And I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "skOrgToken"
    And the response status code should be 200
    When I publish a unique REST API to API Portal "api-portal-multi-organization" with the shared key and header "organization" set to "${CTX:skOrg}" and store its id as "refused"
    Then the response status code should be 403
    When I send an API Portal "GET" request to "/apis/${CTX:refused}" using portal "api-portal-multi-organization" with token "skOrgToken"
    Then the response status code should be 404
    When I send an API Portal "GET" request to "/apis/${CTX:refused}" using portal "api-portal-multi-organization" with token "defaultToken"
    Then the response status code should be 404

    # A shared-key call naming another organization is refused and creates nothing
    Given I generate a unique resource name from "sk-org" and store it as "skOrg"
    And I mint an API Portal IDP token stored as "defaultToken" with claims:
      | sub    | dora         |
      | org_id | default      |
      | roles  | ["ap_admin"] |
    And I mint an API Portal IDP token stored as "skOrgToken" with claims:
      | sub        | sam                       |
      | org_id     | ${CTX:skOrg}              |
      | org_name   | SK Org ${CTX:skOrg}       |
      | org_handle | sk-${CTX:skOrg}           |
      | roles      | ["ap_admin"]              |
    And I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "skOrgToken"
    And the response status code should be 200
    When I publish a unique REST API to API Portal "api-portal-multi-organization" with the shared key and header "organization" set to "no-such-${CTX:skOrg}" and store its id as "refused"
    Then the response status code should be 403
    When I send an API Portal "GET" request to "/apis/${CTX:refused}" using portal "api-portal-multi-organization" with token "skOrgToken"
    Then the response status code should be 404
    When I send an API Portal "GET" request to "/apis/${CTX:refused}" using portal "api-portal-multi-organization" with token "defaultToken"
    Then the response status code should be 404

    # A shared-key call with the wrong key is rejected
    Given I generate a unique resource name from "sk-org" and store it as "skOrg"
    And I mint an API Portal IDP token stored as "defaultToken" with claims:
      | sub    | dora         |
      | org_id | default      |
      | roles  | ["ap_admin"] |
    And I mint an API Portal IDP token stored as "skOrgToken" with claims:
      | sub        | sam                       |
      | org_id     | ${CTX:skOrg}              |
      | org_name   | SK Org ${CTX:skOrg}       |
      | org_handle | sk-${CTX:skOrg}           |
      | roles      | ["ap_admin"]              |
    And I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-organization" with token "skOrgToken"
    And the response status code should be 200
    When I publish a unique REST API to API Portal "api-portal-multi-organization" with shared key "not-the-key" and store its id as "wrong"
    Then the response status code should be 401
