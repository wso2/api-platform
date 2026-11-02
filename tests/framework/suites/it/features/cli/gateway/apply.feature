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

@ap-cli-gateway
Feature: ap CLI gateway apply
  As a CLI user
  I want to apply API and MCP proxy definitions to the gateway
  So that I can deploy APIs and MCP proxies

  Background:
    Given the "ap" CLI is available
    And the CLI uses the platform gateway as "apply-gw"

  @legacy:GW-APPLY-001
  Scenario: Apply a valid API definition
    Given I generate a unique resource name from "cli-rest-api" and store it as "apiName"
    And I generate a unique API context from "/cli-rest-api" and store it as "apiContext"
    When I apply API from "resources/templates/rest-api.yaml" through the CLI with values:
      | apiVersion | ${CTX:gatewaySpecVersion} |
      | name       | ${CTX:apiName}            |
      | spec       | {"displayName":"${CTX:apiName}","version":"v1.0","context":"${CTX:apiContext}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/data"}]} |
    Then the CLI command should succeed
    And the CLI output should contain "applied successfully"
    When I run ap with arguments "gateway rest-api get --id ${CTX:apiName}"
    Then the CLI command should succeed
    And the CLI output should contain "${CTX:apiName}"

  @legacy:GW-APPLY-002
  Scenario: Applying a file that is not valid YAML is rejected
    When I run ap with arguments "gateway apply -f resources/fixtures/ap-cli/malformed-resource.yaml"
    Then the CLI command should fail

  @legacy:GW-APPLY-003
  Scenario: Applying a file that does not exist is rejected
    When I run ap with arguments "gateway apply -f non-existent-file.yaml"
    Then the CLI command should fail
    And the CLI output should contain "no such file"

  @legacy:GW-APPLY-004
  Scenario: Apply a valid MCP proxy definition
    Given I generate a unique resource name from "cli-mcp" and store it as "mcpName"
    And I generate a unique API version from "cli-mcp" and store it as "mcpVersion"
    And I generate a unique API context from "/cli-mcp" and store it as "mcpContext"
    When I apply MCP proxy from "resources/templates/mcp.yaml" through the CLI with values:
      | apiVersion        | ${CTX:gatewaySpecVersion}                         |
      | name              | ${CTX:mcpName}                                    |
      | displayName       | ${CTX:mcpName}                                    |
      | version           | ${CTX:mcpVersion}                                 |
      | context           | ${CTX:mcpContext}                                 |
      | specVersion       | 2025-06-18                                        |
      | spec.upstream.url | http://testbench:3009${CTX:gatewayMCPUpstreamPath} |
    Then the CLI command should succeed
    And the CLI output should contain "applied successfully"
