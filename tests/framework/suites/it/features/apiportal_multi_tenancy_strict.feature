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

Feature: API Portal multi-tenancy with organization validation enforced

  # auth.enforce_org_validation = true: an unknown organization is refused rather than
  # provisioned, and so is a credential that names none.

  Scenario: The configured organization is served
    Given I mint an API Portal IDP token stored as "token" with claims:
      | sub    | alice        |
      | org_id | default      |
      | roles  | ["ap_admin"] |
    When I send an API Portal "GET" request to "/organizations/default" using portal "api-portal-multi-tenancy" with token "token"
    Then the response status code should be 200

  Scenario Outline: A claim differing only in case or trailing spaces matches no organization
    # SQL Server's default collation would match these to "default" in the query itself.
    Given I mint an API Portal IDP token stored as "token" with claims:
      | sub    | c            |
      | org_id | <claim>      |
      | roles  | ["ap_admin"] |
    When I send an API Portal "GET" request to "/organizations/default" using portal "api-portal-multi-tenancy" with token "token"
    Then the response status code should be 403

    Examples:
      | claim      |
      | DEFAULT    |
      | Default    |
      | "default " |

  Scenario: An unknown organization is refused and not provisioned
    Given I generate a unique resource name from "strict-new" and store it as "strictNew"
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub    | x                |
      | org_id | ${CTX:strictNew} |
      | roles  | ["ap_admin"]     |
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-tenancy" with token "token"
    Then the response status code should be 403
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-tenancy" with token "token"
    Then the response status code should be 403
    And the API Portal "api-portal-multi-tenancy" should have 0 organizations with IDP reference "${CTX:strictNew}"

  Scenario: A missing organization claim is refused for a token and for a login
    Given I mint an API Portal IDP token stored as "noClaim" with claims:
      | sub   | y            |
      | roles | ["ap_admin"] |
    And I mint an API Portal IDP token stored as "handleOnly" with claims:
      | sub        | y2           |
      | org_handle | default      |
      | roles      | ["ap_admin"] |
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-tenancy" with token "noClaim"
    Then the response status code should be 403
    When I send an API Portal "GET" request to "/apis" using portal "api-portal-multi-tenancy" with token "handleOnly"
    Then the response status code should be 403
    Given the API Portal browser "z" has an IDP session with claims:
      | sub   | z            |
      | roles | ["ap_admin"] |
    When I sign in to API Portal "api-portal-multi-tenancy" from "/api-portal/default/views/default/login" as browser "z"
    Then the response status code should be 403
