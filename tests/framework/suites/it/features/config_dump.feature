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

@config-dump
Feature: Configuration dump endpoint
  As a gateway administrator
  I want to retrieve the complete gateway configuration snapshot
  So that I can debug and verify deployed APIs, policies, and certificates
  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Config dump has the expected top-level and statistics structure
    When I send a "GET" request to the "gateway-controller-admin" service at "/config_dump"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response should have field "timestamp"
    And the JSON response should have field "apis"
    And the JSON response should have field "policies"
    And the JSON response should have field "certificates"
    And the JSON response should have field "statistics"
    And the JSON response should have field "statistics.totalApis"
    And the JSON response should have field "statistics.totalPolicies"
    And the JSON response should have field "statistics.totalCertificates"

  Scenario: Config dump reflects a deployed API
    Given I generate a unique value from "config-dump-api" and store it as "apiName"
    And I generate a unique API version from "config-dump-api" and store it as "apiVersion"
    And I generate a unique API context from "/config-dump-api" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
    Then the response should be successful

    When I send a "GET" request to the "gateway-controller-admin" service at "/config_dump"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "${CTX:apiName}"
    And the response body should contain "${CTX:apiContext}"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Config dump includes multiple deployed APIs
    Given I generate a unique value from "config-dump-api1" and store it as "api1Name"
    And I generate a unique API version from "config-dump-api1" and store it as "api1Version"
    And I generate a unique API context from "/config-dump-api1" and store it as "api1Context"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:api1Name}                   |
      | spec.displayName       | ${CTX:api1Name}                   |
      | spec.version           | ${CTX:api1Version}                |
      | spec.context           | ${CTX:api1Context}/$version       |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/resource1"}] |
    Then the response should be successful

    Given I generate a unique value from "config-dump-api2" and store it as "api2Name"
    And I generate a unique API version from "config-dump-api2" and store it as "api2Version"
    And I generate a unique API context from "/config-dump-api2" and store it as "api2Context"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:api2Name}                   |
      | spec.displayName       | ${CTX:api2Name}                   |
      | spec.version           | ${CTX:api2Version}                |
      | spec.context           | ${CTX:api2Context}/$version       |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/resource2"}] |
    Then the response should be successful

    When I send a "GET" request to the "gateway-controller-admin" service at "/config_dump"
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should contain "${CTX:api1Name}"
    And the response body should contain "${CTX:api2Name}"

    When I delete the API "${CTX:api1Name}"
    Then the response should be successful
    When I delete the API "${CTX:api2Name}"
    Then the response should be successful

  Scenario: Config dump includes an API with a CORS policy
    Given I generate a unique value from "config-dump-cors" and store it as "apiName"
    And I generate a unique API version from "config-dump-cors" and store it as "apiVersion"
    And I generate a unique API context from "/config-dump-cors" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/test","policies":[{"name":"cors","version":"v1","params":{"allowedOrigins":["http://example.com"],"allowedMethods":["GET","POST"],"allowedHeaders":["Content-Type"],"allowCredentials":true}}]}] |
    Then the response should be successful

    When I send a "GET" request to the "gateway-controller-admin" service at "/config_dump"
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should contain "${CTX:apiContext}"
    And the response body should contain "cors"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Config dump statistics reflect a newly deployed API
    Given I generate a unique value from "config-dump-stats" and store it as "apiName"
    And I generate a unique API version from "config-dump-stats" and store it as "apiVersion"
    And I generate a unique API context from "/config-dump-stats" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
    Then the response should be successful

    When I send a "GET" request to the "gateway-controller-admin" service at "/config_dump"
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should contain "${CTX:apiName}"
    # Exact totals aren't meaningful in a shared block another runner may be using
    # concurrently, but a positive count after our own deployment is.
    And the JSON response field "statistics.totalApis" should be greater than 0

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Config dump includes a deployed MCP proxy
    Given I generate a unique resource name from "config-dump-mcp" and store it as "mcpName"
    And I generate a unique value from "config-dump-mcp" and store it as "mcpDisplayName"
    And I generate a unique API version from "config-dump-mcp" and store it as "mcpVersion"
    And I generate a unique API context from "/config-dump-mcp" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
    Then the response should be successful

    When I send a "GET" request to the "gateway-controller-admin" service at "/config_dump"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "${CTX:mcpName}"

    When I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  Scenario: Config dump includes a deployed LLM provider
    Given I generate a unique resource name from "config-dump-llm" and store it as "providerName"
    And I generate a unique value from "config-dump-llm" and store it as "providerDisplayName"
    And I generate a unique API version from "config-dump-llm" and store it as "providerVersion"
    And I generate a unique API context from "/config-dump-llm" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}                |
      | displayName        | ${CTX:providerDisplayName}         |
      | version            | ${CTX:providerVersion}              |
      | template           | openai                              |
      | spec.context       | ${CTX:providerContext}              |
      | spec.upstream.url  | http://testbench:3008/anything      |
      | accessControl.mode | allow_all                           |
    Then the response should be successful

    When I send a "GET" request to the "gateway-controller-admin" service at "/config_dump"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "${CTX:providerName}"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: Config dump includes mixed resource types together
    Given I generate a unique value from "config-dump-mixed-api" and store it as "apiName"
    And I generate a unique API version from "config-dump-mixed-api" and store it as "apiVersion"
    And I generate a unique API context from "/config-dump-mixed-api" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/test"}] |
    Then the response should be successful

    Given I generate a unique resource name from "config-dump-mixed-mcp" and store it as "mcpName"
    And I generate a unique value from "config-dump-mixed-mcp" and store it as "mcpDisplayName"
    And I generate a unique API version from "config-dump-mixed-mcp" and store it as "mcpVersion"
    And I generate a unique API context from "/config-dump-mixed-mcp" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
    Then the response should be successful

    Given I generate a unique resource name from "config-dump-mixed-llm" and store it as "providerName"
    And I generate a unique value from "config-dump-mixed-llm" and store it as "providerDisplayName"
    And I generate a unique API version from "config-dump-mixed-llm" and store it as "providerVersion"
    And I generate a unique API context from "/config-dump-mixed-llm" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}                |
      | displayName        | ${CTX:providerDisplayName}         |
      | version            | ${CTX:providerVersion}              |
      | template           | openai                              |
      | spec.context       | ${CTX:providerContext}              |
      | spec.upstream.url  | http://testbench:3008/anything      |
      | accessControl.mode | allow_all                           |
    Then the response should be successful

    When I send a "GET" request to the "gateway-controller-admin" service at "/config_dump"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "${CTX:apiName}"
    And the response body should contain "${CTX:mcpName}"
    And the response body should contain "${CTX:providerName}"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
    When I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: Config dump reflects a removed API after deletion
    Given I generate a unique value from "config-dump-deletion" and store it as "apiName"
    And I generate a unique API version from "config-dump-deletion" and store it as "apiVersion"
    And I generate a unique API context from "/config-dump-deletion" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
    Then the response should be successful

    When I send a "GET" request to the "gateway-controller-admin" service at "/config_dump"
    Then the response status should be 200
    And the response body should contain "${CTX:apiName}"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

    When I send a "GET" request to the "gateway-controller-admin" service at "/config_dump" until the response body does not contain "${CTX:apiName}"
