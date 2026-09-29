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

Feature: API Portal multi-tenancy webhook delivery

  # The dispatcher and delivery worker of a multi-tenancy portal handle every organization
  # under its portal_id; each organization's subscribers still receive only its own events.

  Scenario: Each organization's subscribers receive exactly that organization's events, once
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
    And an API Portal webhook subscriber for events "apikey.*" delivering to sink "a" is registered in portal "api-portal-multi-tenancy" with token "tokenA"
    And an API Portal webhook subscriber for events "apikey.*" delivering to sink "b" is registered in portal "api-portal-multi-tenancy" with token "tokenB"
    And a unique API Portal REST API is created in portal "api-portal-multi-tenancy" with token "tokenA" and stored as "apiA"
    And a unique API Portal REST API is created in portal "api-portal-multi-tenancy" with token "tokenB" and stored as "apiB"
    And I store the API Portal "api-portal-multi-tenancy" organization id for IDP reference "${CTX:hookA}" as "orgA"
    And I store the API Portal "api-portal-multi-tenancy" organization id for IDP reference "${CTX:hookB}" as "orgB"
    When I generate 2 API Portal API keys for API "apiA" in portal "api-portal-multi-tenancy" with token "tokenA"
    And I generate 2 API Portal API keys for API "apiB" in portal "api-portal-multi-tenancy" with token "tokenB"
    Then the API Portal webhook sink "a" should receive exactly 2 "apikey.generated" events for organization "${CTX:orgA}"
    And the API Portal webhook sink "b" should receive exactly 2 "apikey.generated" events for organization "${CTX:orgB}"

  Scenario: Concurrent work across two replicas of one deployment is delivered exactly once
    Given I generate a unique resource name from "replicated" and store it as "replicated"
    And I mint an API Portal IDP token stored as "token" with claims:
      | sub      | rep                           |
      | org_id   | ${CTX:replicated}             |
      | org_name | Replicated ${CTX:replicated}  |
      | roles    | ["ap_admin"]                  |
    And an API Portal webhook subscriber for events "apikey.*" delivering to sink "replicated" is registered in portal "api-portal-multi-tenancy" with token "token"
    And a unique API Portal REST API is created in portal "api-portal-multi-tenancy" with token "token" and stored as "api"
    And I store the API Portal "api-portal-multi-tenancy" organization id for IDP reference "${CTX:replicated}" as "org"
    When I generate 20 API Portal API keys for API "api" concurrently across portals "api-portal-multi-tenancy" and "api-portal-multi-tenancy-replica" with token "token"
    Then the API Portal webhook sink "replicated" should receive exactly 20 "apikey.generated" events for organization "${CTX:org}"
