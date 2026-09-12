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

@dp-to-cp-reconnect
Feature: Rejected control-plane pushes are retried on gateway-controller reconnect
  As a platform operator
  I want a gateway-originated push that the control plane rejected to be retried once
  gateway-controller reconnects
  So that a transient rejection (e.g. a not-yet-created project) does not permanently lose the
  artifact

  # Restarts gateway-controller, so this lives in its own runner rather than alongside
  # dp-to-cp's other scenarios in platform-api-gateway - those run concurrently with sibling
  # runners that expect gateway-controller continuously reachable, and a restart racing their
  # polling windows caused an intermittent connection-refused failure unrelated to this
  # scenario's own behavior.
  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "dp2cp-reconnect-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

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
