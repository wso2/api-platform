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

Feature: API Portal multi-tenancy tenant isolation

  # The org claims deliberately differ from the handles the organization names derive.
  Background:
    Given I generate a unique resource name from "iso-a" and store it as "isoA"
    And I generate a unique resource name from "iso-b" and store it as "isoB"
    And I mint an API Portal IDP token stored as "tokenA" with claims:
      | sub      | ann                  |
      | org_id   | ${CTX:isoA}          |
      | org_name | Tenant A ${CTX:isoA} |
      | roles    | ["ap_admin"]         |
    And I mint an API Portal IDP token stored as "tokenB" with claims:
      | sub      | ben                  |
      | org_id   | ${CTX:isoB}          |
      | org_name | Tenant B ${CTX:isoB} |
      | roles    | ["ap_admin"]         |

  Scenario: An organization never sees another organization's APIs
    Given a unique API Portal REST API is created in portal "api-portal-multi-tenancy" with token "tokenA" and stored as "api"
    When I send an API Portal "GET" request to "/apis/${CTX:api}" using portal "api-portal-multi-tenancy" with token "tokenA"
    Then the response status code should be 200
    When I send an API Portal "GET" request to "/apis/${CTX:api}" using portal "api-portal-multi-tenancy" with token "tokenB"
    Then the response status code should be 404
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-tenancy" with token "tokenB"
    Then the response status code should be 200
    And the response body should not contain "${CTX:api}"
    When I send an API Portal "POST" request to "/apis/${CTX:api}/api-keys/generate" using portal "api-portal-multi-tenancy" with token "tokenB" and JSON body:
      """
      {"id": "stolen"}
      """
    Then the response status code should be 404

  Scenario: The organization APIs accept only the caller's own organization
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-tenancy" with token "tokenB"
    Then the response status code should be 200
    When I send an API Portal "GET" request to "/organizations/tenant-a-${CTX:isoA}" using portal "api-portal-multi-tenancy" with token "tokenA"
    Then the response status code should be 200
    When I send an API Portal "GET" request to "/organizations/tenant-b-${CTX:isoB}" using portal "api-portal-multi-tenancy" with token "tokenA"
    Then the response status code should be 403
    When I send an API Portal "GET" request to "/organizations/default" using portal "api-portal-multi-tenancy" with token "tokenA"
    Then the response status code should be 403
    When I send an API Portal "PUT" request to "/organizations/default" using portal "api-portal-multi-tenancy" with token "tokenA" and JSON body:
      """
      {"id": "default", "displayName": "taken over", "idpRefId": "default"}
      """
    Then the response status code should be 403

  Scenario: The MCP registry's write endpoints act only in the caller's own organization
    Given a unique API Portal MCP server is created in portal "api-portal-multi-tenancy" with token "tokenA" and stored as "mcp"
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-tenancy" with token "tokenB"
    Then the response status code should be 200
    When I send an API Portal "DELETE" request to "/api-portal/registry/tenant-a-${CTX:isoA}/v0.1/servers/${CTX:mcp}/versions/1.0.0" using portal "api-portal-multi-tenancy" with token "tokenB"
    Then the response status code should be 403
    When I send an API Portal "PATCH" request to "/api-portal/registry/tenant-a-${CTX:isoA}/v0.1/servers/${CTX:mcp}/status" using portal "api-portal-multi-tenancy" with token "tokenB" and JSON body:
      """
      {"status": "deprecated"}
      """
    Then the response status code should be 403
    When I send an unauthenticated API Portal "GET" request to "/api-portal/registry/tenant-a-${CTX:isoA}/v0.1/servers" using portal "api-portal-multi-tenancy"
    Then the response status code should be 200
    And the response body should contain "${CTX:mcp}"
    When I send an API Portal "DELETE" request to "/api-portal/registry/tenant-a-${CTX:isoA}/v0.1/servers/${CTX:mcp}/versions/1.0.0" using portal "api-portal-multi-tenancy" with token "tokenA"
    Then the response status code should be 200

  Scenario: A claim is matched on idp_ref_id only, never on a handle
    Given I mint an API Portal IDP token stored as "tokenHandle" with claims:
      | sub    | h                     |
      | org_id | tenant-a-${CTX:isoA}  |
      | roles  | ["ap_admin"]          |
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-tenancy" with token "tokenA"
    Then the response status code should be 200
    When I send an API Portal "GET" request to "/organizations/tenant-a-${CTX:isoA}" using portal "api-portal-multi-tenancy" with token "tokenHandle"
    Then the response status code should be 403
    And the API Portal "api-portal-multi-tenancy" organization with IDP reference "tenant-a-${CTX:isoA}" should have a handle matching "tenant-a-${CTX:isoA}-[0-9a-f]{6}"
