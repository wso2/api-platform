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

Feature: API Portal multi-organization publishing with the shared key

  # The portal holds one shared key for every platform-api caller, so a shared-key call cannot
  # choose an organization: it publishes into the configured one.

  Background:
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

  Scenario: A shared-key call publishes into the configured organization
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

  Scenario Outline: A shared-key call naming another organization is refused and creates nothing
    When I publish a unique REST API to API Portal "api-portal-multi-organization" with the shared key and header "organization" set to "<organization>" and store its id as "refused"
    Then the response status code should be 403
    When I send an API Portal "GET" request to "/apis/${CTX:refused}" using portal "api-portal-multi-organization" with token "skOrgToken"
    Then the response status code should be 404
    When I send an API Portal "GET" request to "/apis/${CTX:refused}" using portal "api-portal-multi-organization" with token "defaultToken"
    Then the response status code should be 404

    Examples:
      | organization           |
      | sk-${CTX:skOrg}        |
      | ${CTX:skOrg}           |
      | no-such-${CTX:skOrg}   |

  Scenario: A shared-key call with the wrong key is rejected
    When I publish a unique REST API to API Portal "api-portal-multi-organization" with shared key "not-the-key" and store its id as "wrong"
    Then the response status code should be 401
