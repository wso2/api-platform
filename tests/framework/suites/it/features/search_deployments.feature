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
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

@search-deployments
Feature: Deployment search
  As an API administrator
  I want to search for deployed APIs and MCP proxies with filters
  So that I can find specific deployments easily

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Search APIs with no filters returns all APIs
    Given I generate a unique value from "search-api-1" and store it as "resourceName1_1"
    Given I generate a unique API context from "/search-one" and store it as "resourceContext1_1"
    Given I generate a unique value from "search-api-2" and store it as "resourceName1_2"
    Given I generate a unique API context from "/search-two" and store it as "resourceContext1_2"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion              | gateway.api-platform.wso2.com/v1 |
      | name                    | ${CTX:resourceName1_1}            |
      | spec.displayName        | Search-API-One                     |
      | spec.version            | v1.0                               |
      | spec.context            | ${CTX:resourceContext1_1}          |
      | spec.upstream.main.url  | http://testbench:3000             |
      | spec.operations         | [{"method":"GET","path":"/test"}] |
    Then the response should be successful
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion              | gateway.api-platform.wso2.com/v1 |
      | name                    | ${CTX:resourceName1_2}            |
      | spec.displayName        | Search-API-Two                     |
      | spec.version            | v2.0                               |
      | spec.context            | ${CTX:resourceContext1_2}          |
      | spec.upstream.main.url  | http://testbench:3000             |
      | spec.operations         | [{"method":"GET","path":"/test"}] |
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "${CTX:resourceName1_1}"
    And the response body should contain "${CTX:resourceName1_2}"
    When I delete the API "${CTX:resourceName1_1}"
    Then the response should be successful
    When I delete the API "${CTX:resourceName1_2}"
    Then the response should be successful

  Scenario: Search APIs by displayName filter
    Given I generate a unique value from "displayname-search-api" and store it as "resourceName2_1"
    Given I generate a unique API context from "/displayname-search" and store it as "resourceContext2_1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion              | gateway.api-platform.wso2.com/v1 |
      | name                    | ${CTX:resourceName2_1}            |
      | spec.displayName        | UniqueDisplayName                  |
      | spec.version            | v1.0                               |
      | spec.context            | ${CTX:resourceContext2_1}          |
      | spec.upstream.main.url  | http://testbench:3000             |
      | spec.operations         | [{"method":"GET","path":"/test"}] |
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis?displayName=UniqueDisplayName"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "UniqueDisplayName"
    When I delete the API "${CTX:resourceName2_1}"
    Then the response should be successful

  Scenario: Search APIs by version filter
    Given I generate a unique value from "version-search-api" and store it as "resourceName3_1"
    Given I generate a unique API context from "/version-search" and store it as "resourceContext3_1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion              | gateway.api-platform.wso2.com/v1 |
      | name                    | ${CTX:resourceName3_1}            |
      | spec.displayName        | Version-Search-API                 |
      | spec.version            | v3.0.0                             |
      | spec.context            | ${CTX:resourceContext3_1}          |
      | spec.upstream.main.url  | http://testbench:3000             |
      | spec.operations         | [{"method":"GET","path":"/test"}] |
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis?version=v3.0.0"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "Version-Search-API"
    When I delete the API "${CTX:resourceName3_1}"
    Then the response should be successful

  Scenario: Search APIs by context filter
    Given I generate a unique value from "context-search-api" and store it as "resourceName4_1"
    Given I generate a unique API context from "/unique-context-path" and store it as "resourceContext4_1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion              | gateway.api-platform.wso2.com/v1 |
      | name                    | ${CTX:resourceName4_1}            |
      | spec.displayName        | Context-Search-API                 |
      | spec.version            | v1.0                               |
      | spec.context            | ${CTX:resourceContext4_1}          |
      | spec.upstream.main.url  | http://testbench:3000             |
      | spec.operations         | [{"method":"GET","path":"/test"}] |
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis?context=${CTX:resourceContext4_1}"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "Context-Search-API"
    When I delete the API "${CTX:resourceName4_1}"
    Then the response should be successful

  Scenario: Search APIs by status filter
    Given I generate a unique value from "status-search-api" and store it as "resourceName5_1"
    Given I generate a unique API context from "/status-search" and store it as "resourceContext5_1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion              | gateway.api-platform.wso2.com/v1 |
      | name                    | ${CTX:resourceName5_1}            |
      | spec.displayName        | Status-Search-API                  |
      | spec.version            | v1.0                               |
      | spec.context            | ${CTX:resourceContext5_1}          |
      | spec.upstream.main.url  | http://testbench:3000             |
      | spec.operations         | [{"method":"GET","path":"/test"}] |
    Then the response should be successful
    And I wait for policy snapshot sync
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis?status=deployed"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "Status-Search-API"
    And the response body should contain "deployed"
    When I delete the API "${CTX:resourceName5_1}"
    Then the response should be successful

  Scenario: Search APIs with multiple filters
    Given I generate a unique value from "multi-filter-api" and store it as "resourceName6_1"
    Given I generate a unique API context from "/multi-filter" and store it as "resourceContext6_1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion              | gateway.api-platform.wso2.com/v1 |
      | name                    | ${CTX:resourceName6_1}            |
      | spec.displayName        | MultiFilterAPI                      |
      | spec.version            | v5.0                               |
      | spec.context            | ${CTX:resourceContext6_1}          |
      | spec.upstream.main.url  | http://testbench:3000             |
      | spec.operations         | [{"method":"GET","path":"/test"}] |
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis?displayName=MultiFilterAPI&version=v5.0"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "MultiFilterAPI"
    When I delete the API "${CTX:resourceName6_1}"
    Then the response should be successful

  Scenario: Search APIs with non-matching filter returns empty
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis?displayName=NonExistentAPIName12345"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response field "count" should be 0

  Scenario: Search MCP proxies with no filters
    Given I generate a unique value from "search-mcp-v1.0" and store it as "mcpName1"
    And I generate a unique API context from "/search-mcp" and store it as "mcpContext1"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion    | gateway.api-platform.wso2.com/v1 |
      | name          | ${CTX:mcpName1}                  |
      | displayName   | SearchMCP                         |
      | version       | v1.0                              |
      | context       | ${CTX:mcpContext1}                |
      | specVersion   | 2025-06-18                        |
      | spec.upstream.url  | http://testbench:3009/mcp        |
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/mcp-proxies"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "SearchMCP"
    And the response body should contain "mcpProxies"
    When I delete the MCP proxy "${CTX:mcpName1}"
    Then the response should be successful

  Scenario: Search MCP proxies by displayName filter
    Given I generate a unique value from "displayname-mcp-v1.0" and store it as "mcpName2"
    And I generate a unique API context from "/displayname-mcp" and store it as "mcpContext2"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion   | gateway.api-platform.wso2.com/v1 |
      | name         | ${CTX:mcpName2}                  |
      | displayName  | UniqueMCPDisplayName             |
      | version      | v1.0                              |
      | context      | ${CTX:mcpContext2}                |
      | specVersion  | 2025-06-18                        |
      | spec.upstream.url | http://testbench:3009/mcp        |
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/mcp-proxies?displayName=UniqueMCPDisplayName"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "UniqueMCPDisplayName"
    When I delete the MCP proxy "${CTX:mcpName2}"
    Then the response should be successful

  Scenario: Search MCP proxies by version filter
    Given I generate a unique value from "version-mcp-v2.0" and store it as "mcpName3"
    And I generate a unique API context from "/version-mcp" and store it as "mcpContext3"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion   | gateway.api-platform.wso2.com/v1 |
      | name         | ${CTX:mcpName3}                  |
      | displayName  | VersionMCP                         |
      | version      | v2.0                              |
      | context      | ${CTX:mcpContext3}                |
      | specVersion  | 2025-06-18                        |
      | spec.upstream.url | http://testbench:3009/mcp        |
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/mcp-proxies?version=v2.0"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "VersionMCP"
    When I delete the MCP proxy "${CTX:mcpName3}"
    Then the response should be successful

  Scenario: Search MCP proxies with non-matching filter
    When I send a "GET" request to the "gateway-controller" service at "/mcp-proxies?displayName=NonExistentMCP12345"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response field "count" should be 0
