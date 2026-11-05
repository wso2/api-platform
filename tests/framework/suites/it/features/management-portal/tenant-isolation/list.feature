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

@TENANT
Feature: Another organization's listings never include an organization's resources
  As an organization administrator
  I want every listing another organization makes to leave out my resources
  So that it cannot discover what my organization holds

  Background:
    Given I generate a unique value from "tenant-list-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  @TENANT-002
  Scenario: Another organization lists REST APIs, gateways, applications, secrets, LLM providers and MCP proxies without any of the organization's
    Given I generate a unique resource name from "tenant-list-api" and store it as "apiHandle"
    And I generate a unique API context from "/tenant-list-api" and store it as "apiContext"
    And I create a REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}"
    And I generate a unique resource name from "tenant-list-gateway" and store it as "gatewayHandle"
    And I register a gateway "${CTX:gatewayHandle}" on the control plane
    And I generate a unique resource name from "tenant-list-app" and store it as "appHandle"
    And I create an application "${CTX:appHandle}" via the control plane in project "${CTX:projectHandle}"
    And I generate a unique resource name from "tenant-list-secret" and store it as "secretHandle"
    And I create a secret "${CTX:secretHandle}" via the control plane
    And I generate a unique resource name from "tenant-list-provider" and store it as "providerHandle"
    And I generate a unique API context from "/tenant-list-provider" and store it as "providerContext"
    And I create an LLM provider "${CTX:providerHandle}" via the control plane with context "${CTX:providerContext}" referencing template "openai"
    And I generate a unique resource name from "tenant-list-mcp" and store it as "mcpHandle"
    And I generate a unique API context from "/tenant-list-mcp" and store it as "mcpContext"
    And I create an MCP proxy "${CTX:mcpHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:mcpContext}" and spec versions "2025-06-18"
    When I send a "GET" request to the control plane at "/rest-apis?projectId=${CTX:projectHandle}"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:apiHandle}"
    When I send a "GET" request to the control plane at "/rest-apis?projectId=default" as "org B admin"
    Then the response status code should be 200
    And the response body should not contain "${CTX:apiHandle}"
    When I send a "GET" request to the control plane at "/rest-apis?projectId=${CTX:projectHandle}" as "org B admin"
    Then the response status code should be 404
    And the JSON response field "code" should be "PROJECT_NOT_FOUND"
    When I send a "GET" request to the control plane at "/gateways"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:gatewayHandle}"
    When I send a "GET" request to the control plane at "/gateways" as "org B admin"
    Then the response status code should be 200
    And the response body should not contain "${CTX:gatewayHandle}"
    When I send a "GET" request to the control plane at "/applications?projectId=${CTX:projectHandle}"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:appHandle}"
    When I send a "GET" request to the control plane at "/applications?projectId=default" as "org B admin"
    Then the response status code should be 200
    And the response body should not contain "${CTX:appHandle}"
    When I send a "GET" request to the control plane at "/applications?projectId=${CTX:projectHandle}" as "org B admin"
    Then the response status code should be 404
    And the JSON response field "code" should be "PROJECT_NOT_FOUND"
    When I send a "GET" request to the control plane at "/secrets"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:secretHandle}"
    When I send a "GET" request to the control plane at "/secrets" as "org B admin"
    Then the response status code should be 200
    And the response body should not contain "${CTX:secretHandle}"
    When I send a "GET" request to the control plane at "/llm-providers"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:providerHandle}"
    When I send a "GET" request to the control plane at "/llm-providers" as "org B admin"
    Then the response status code should be 200
    And the response body should not contain "${CTX:providerHandle}"
    When I send a "GET" request to the control plane at "/mcp-proxies?projectId=${CTX:projectHandle}"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:mcpHandle}"
    When I send a "GET" request to the control plane at "/mcp-proxies?projectId=default" as "org B admin"
    Then the response status code should be 200
    And the response body should not contain "${CTX:mcpHandle}"
