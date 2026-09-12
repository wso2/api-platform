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

@dp-to-cp-sync-disabled
Feature: Data-plane to control-plane push is suppressed when deployment sync is disabled
  As a platform operator
  I want to disable the gateway's push to the control plane
  So that a gateway can run standalone against a control plane without synchronising artifacts

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: An LLM provider template is not pushed when sync is disabled
    Given I generate a unique resource name from "nosync-tmpl" and store it as "templateName"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:templateName}               |
      | displayName        | ${CTX:templateName}               |
      | spec.requestModel  | {"location":"payload","identifier":"$.model"} |
      | spec.responseModel | {"location":"payload","identifier":"$.model"} |
    Then the response should be successful
    And the control plane should not receive the "LlmProviderTemplate" artifact "${CTX:templateName}"

  Scenario: An LLM provider and proxy are not pushed when sync is disabled
    Given I generate a unique resource name from "nosync-chain-tmpl" and store it as "templateName"
    And I generate a unique resource name from "nosync-chain-prov" and store it as "providerName"
    And I generate a unique value from "nosync-chain-prov-display" and store it as "providerDisplayName"
    And I generate a unique API version from "nosync-chain-prov" and store it as "providerVersion"
    And I generate a unique resource name from "nosync-chain-proxy" and store it as "proxyName"
    And I generate a unique value from "nosync-chain-proxy-display" and store it as "proxyDisplayName"
    And I generate a unique API version from "nosync-chain-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/nosync-chain-proxy" and store it as "proxyContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:templateName}               |
      | displayName        | ${CTX:templateName}               |
      | spec.requestModel  | {"location":"payload","identifier":"$.model"} |
      | spec.responseModel | {"location":"payload","identifier":"$.model"} |
    Then the response should be successful

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.upstream.url  | http://testbench:3008/openai/v1   |
      | accessControl.mode | allow_all                          |
    Then the response should be successful

    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:proxyName}                  |
      | displayName | ${CTX:proxyDisplayName}           |
      | version     | ${CTX:proxyVersion}               |
      | context     | ${CTX:proxyContext}               |
      | provider.id | ${CTX:providerName}                |
    Then the response should be successful
    And the control plane should not receive the "LlmProvider" artifact "${CTX:providerName}"
    And the control plane should not receive the "LlmProxy" artifact "${CTX:proxyName}"

  Scenario: An MCP proxy is not pushed when sync is disabled
    Given I generate a unique resource name from "nosync-mcp" and store it as "mcpName"
    And I generate a unique value from "nosync-mcp-display" and store it as "mcpDisplayName"
    And I generate a unique API version from "nosync-mcp" and store it as "mcpVersion"
    And I generate a unique API context from "/nosync-mcp" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                          |
      | spec.upstream.url | http://testbench:3009/mcp          |
    Then the response should be successful
    And the control plane should not receive the "Mcp" artifact "${CTX:mcpName}"

  Scenario: A REST API is not pushed when sync is disabled
    Given I generate a unique value from "nosync-rest" and store it as "apiName"
    And I generate a unique value from "nosync-rest-display" and store it as "apiDisplayName"
    And I generate a unique API version from "nosync-rest" and store it as "apiVersion"
    And I generate a unique API context from "/nosync-rest" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                     |
      | spec.displayName       | ${CTX:apiDisplayName}              |
      | spec.version           | ${CTX:apiVersion}                  |
      | spec.context           | ${CTX:apiContext}/$version         |
      | spec.upstream.main.url | http://testbench:3000/api/v2        |
      | spec.operations        | [{"method":"GET","path":"/resource"}] |
    Then the response should be successful
    And the control plane should not receive the "RestApi" artifact "${CTX:apiName}"
