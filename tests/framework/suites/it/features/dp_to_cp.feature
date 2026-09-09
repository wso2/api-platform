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

@dp-to-cp
Feature: Data-plane to control-plane artifact push
  As a platform operator
  I want the gateway to push gateway-originated artifacts to the control plane
  So that artifacts created directly on a gateway are visible and manageable centrally

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "dp2cp-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  Scenario: An LLM provider template created on the gateway is pushed to the control plane
    Given I generate a unique resource name from "dp2cp-tmpl" and store it as "templateName"
    And I generate a unique value from "dp2cp-tmpl-display" and store it as "templateDisplayName"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:templateName}               |
      | displayName        | ${CTX:templateDisplayName}        |
      | spec.requestModel  | {"location":"payload","identifier":"$.model"} |
      | spec.responseModel | {"location":"payload","identifier":"$.model"} |
    Then the response should be successful
    And the control plane should receive the "LlmProviderTemplate" artifact "${CTX:templateName}"
    And the control plane copy of the "LlmProviderTemplate" artifact "${CTX:templateName}" configuration should contain "${CTX:templateDisplayName}"
    And the control plane copy of the "LlmProviderTemplate" artifact "${CTX:templateName}" should be marked as gateway-originated

  Scenario: An LLM provider and proxy are pushed with their cross-references carried as handles
    Given I generate a unique resource name from "dp2cp-chain-tmpl" and store it as "templateName"
    And I generate a unique value from "dp2cp-chain-tmpl-display" and store it as "templateDisplayName"
    And I generate a unique resource name from "dp2cp-chain-prov" and store it as "providerName"
    And I generate a unique value from "dp2cp-chain-prov-display" and store it as "providerDisplayName"
    And I generate a unique API version from "dp2cp-chain-prov" and store it as "providerVersion"
    And I generate a unique resource name from "dp2cp-chain-proxy" and store it as "proxyName"
    And I generate a unique value from "dp2cp-chain-proxy-display" and store it as "proxyDisplayName"
    And I generate a unique API version from "dp2cp-chain-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/dp2cp-chain-proxy" and store it as "proxyContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:templateName}               |
      | displayName        | ${CTX:templateDisplayName}        |
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
    And the control plane should receive the "LlmProvider" artifact "${CTX:providerName}"
    And the control plane copy of the "LlmProvider" artifact "${CTX:providerName}" should reference template "${CTX:templateName}"

    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion          | gateway.api-platform.wso2.com/v1 |
      | name                | ${CTX:proxyName}                  |
      | displayName         | ${CTX:proxyDisplayName}           |
      | version             | ${CTX:proxyVersion}               |
      | context             | ${CTX:proxyContext}               |
      | provider.id         | ${CTX:providerName}                |
      | metadata.annotations | {"gateway.api-platform.wso2.com/project-id":"${CTX:projectHandle}"} |
    Then the response should be successful
    And the control plane should receive the "LlmProxy" artifact "${CTX:proxyName}"
    And the control plane copy of the "LlmProxy" artifact "${CTX:proxyName}" should reference provider "${CTX:providerName}"

  Scenario: An MCP proxy created on the gateway is pushed to the control plane
    Given I generate a unique resource name from "dp2cp-mcp" and store it as "mcpName"
    And I generate a unique value from "dp2cp-mcp-display" and store it as "mcpDisplayName"
    And I generate a unique API version from "dp2cp-mcp" and store it as "mcpVersion"
    And I generate a unique API context from "/dp2cp-mcp" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1 |
      | name                  | ${CTX:mcpName}                    |
      | displayName           | ${CTX:mcpDisplayName}             |
      | version               | ${CTX:mcpVersion}                 |
      | context               | ${CTX:mcpContext}                 |
      | specVersion           | 2025-06-18                          |
      | spec.upstream.url     | http://testbench:3009/mcp          |
      | metadata.annotations  | {"gateway.api-platform.wso2.com/project-id":"${CTX:projectHandle}"} |
    Then the response should be successful
    And the control plane should receive the "Mcp" artifact "${CTX:mcpName}"
    And the control plane copy of the "Mcp" artifact "${CTX:mcpName}" configuration should contain "${CTX:mcpContext}"

  Scenario: A REST API created on the gateway is pushed to the control plane
    Given I generate a unique value from "dp2cp-rest" and store it as "apiName"
    And I generate a unique value from "dp2cp-rest-display" and store it as "apiDisplayName"
    And I generate a unique API version from "dp2cp-rest" and store it as "apiVersion"
    And I generate a unique API context from "/dp2cp-rest" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                     |
      | spec.displayName       | ${CTX:apiDisplayName}              |
      | spec.version           | ${CTX:apiVersion}                  |
      | spec.context           | ${CTX:apiContext}/$version         |
      | spec.upstream.main.url | http://testbench:3000/api/v2        |
      | spec.operations        | [{"method":"GET","path":"/resource"}] |
      | metadata.annotations   | {"gateway.api-platform.wso2.com/project-id":"${CTX:projectHandle}"} |
    Then the response should be successful
    And the control plane should receive the "RestApi" artifact "${CTX:apiName}"
    And the control plane copy of the "RestApi" artifact "${CTX:apiName}" configuration should contain "${CTX:apiContext}"

  Scenario: Updating a gateway-originated LLM provider pushes a fresh deployment to the control plane
    Given I generate a unique resource name from "dp2cp-upd-tmpl" and store it as "templateName"
    And I generate a unique resource name from "dp2cp-upd-prov" and store it as "providerName"
    And I generate a unique value from "dp2cp-upd-prov-display" and store it as "providerDisplayName"
    And I generate a unique API version from "dp2cp-upd-prov" and store it as "providerVersion"
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
    And the control plane should receive the "LlmProvider" artifact "${CTX:providerName}"

    When I update LLM provider "${CTX:providerName}" from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName} Updated |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.upstream.url  | http://testbench:3008/openai/v1   |
      | accessControl.mode | allow_all                          |
    Then the response should be successful
    And the control plane copy of the "LlmProvider" artifact "${CTX:providerName}" configuration should contain "${CTX:providerDisplayName} Updated"

  Scenario: Updating a gateway-originated LLM provider template re-pushes it to the control plane
    Given I generate a unique resource name from "dp2cp-tmpl-upd" and store it as "templateName"
    And I generate a unique value from "dp2cp-tmpl-upd-display" and store it as "templateDisplayNameBefore"
    And I generate a unique value from "dp2cp-tmpl-upd-display-after" and store it as "templateDisplayNameAfter"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:templateName}               |
      | displayName        | ${CTX:templateDisplayNameBefore}  |
      | spec.requestModel  | {"location":"payload","identifier":"$.model"} |
      | spec.responseModel | {"location":"payload","identifier":"$.model"} |
    Then the response should be successful
    And the control plane should receive the "LlmProviderTemplate" artifact "${CTX:templateName}"

    When I update LLM provider template "${CTX:templateName}" from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:templateName}               |
      | displayName        | ${CTX:templateDisplayNameAfter}   |
      | spec.requestModel  | {"location":"payload","identifier":"$.model"} |
      | spec.responseModel | {"location":"payload","identifier":"$.model"} |
    Then the response should be successful
    And the control plane copy of the "LlmProviderTemplate" artifact "${CTX:templateName}" configuration should contain "${CTX:templateDisplayNameAfter}"
    And the control plane copy of the "LlmProviderTemplate" artifact "${CTX:templateName}" should be marked as gateway-originated

  Scenario: Deleting a gateway-originated artifact undeploys it on the control plane
    Given I generate a unique resource name from "dp2cp-mcp-del" and store it as "mcpName"
    And I generate a unique value from "dp2cp-mcp-del-display" and store it as "mcpDisplayName"
    And I generate a unique API version from "dp2cp-mcp-del" and store it as "mcpVersion"
    And I generate a unique API context from "/dp2cp-mcp-del" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1 |
      | name                  | ${CTX:mcpName}                    |
      | displayName           | ${CTX:mcpDisplayName}             |
      | version               | ${CTX:mcpVersion}                 |
      | context               | ${CTX:mcpContext}                 |
      | specVersion           | 2025-06-18                          |
      | spec.upstream.url     | http://testbench:3009/mcp          |
      | metadata.annotations  | {"gateway.api-platform.wso2.com/project-id":"${CTX:projectHandle}"} |
    Then the response should be successful
    And the control plane should receive the "Mcp" artifact "${CTX:mcpName}"
    And the control plane should have deployed the "Mcp" artifact "${CTX:mcpName}"

    When I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful
    And the control plane should have undeployed the "Mcp" artifact "${CTX:mcpName}"

  Scenario: A push rejected by the control plane is recorded as failed and re-pushed on reconnect
    Given I generate a unique resource name from "dp2cp-reject" and store it as "mcpName"
    And I generate a unique value from "dp2cp-reject-display" and store it as "mcpDisplayName"
    And I generate a unique API version from "dp2cp-reject" and store it as "mcpVersion"
    And I generate a unique API context from "/dp2cp-reject" and store it as "mcpContext"
    And I generate a unique value from "dp2cp-missing-project" and store it as "missingProjectHandle"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1 |
      | name                  | ${CTX:mcpName}                    |
      | displayName           | ${CTX:mcpDisplayName}             |
      | version               | ${CTX:mcpVersion}                 |
      | context               | ${CTX:mcpContext}                 |
      | specVersion           | 2025-06-18                          |
      | spec.upstream.url     | http://testbench:3009/mcp          |
      | metadata.annotations  | {"gateway.api-platform.wso2.com/project-id":"${CTX:missingProjectHandle}"} |
    Then the response should be successful
    And the control plane should not receive the "Mcp" artifact "${CTX:mcpName}"

    When I create a project "${CTX:missingProjectHandle}" on the control plane
    And I restart the "gateway-controller" service
    Then the control plane should receive the "Mcp" artifact "${CTX:mcpName}"
    And the control plane should have deployed the "Mcp" artifact "${CTX:mcpName}"
