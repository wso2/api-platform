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

@ai-workspace-cli
Feature: AI Workspace CLI publish and persistence
  As a platform user driving the `ap` CLI
  I want to publish AI Workspace artifacts to platform-api
  So that I can confirm the CLI create/update/read behaviour persists on the backend

  The suite boots a real platform-api (the AI Workspace backend), authenticates as the
  administrator for a bearer token, creates a server-side project, and registers the
  gateways the artifacts associate with. Each scenario scaffolds a project with
  `ap project init`, applies the "create" demo content, reads it back with both
  `get` and `list`, then applies the "edit" demo content (which adds
  spec.associatedGateways) to confirm the update path persists too.

  Background:
    Given the "ap" CLI is available
    And I generate a unique value from "awcli-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane
    And I generate a unique resource name from "awcli-gateway-a" and store it as "gatewayHandleA"
    And I register a gateway "${CTX:gatewayHandleA}" on the control plane
    And I generate a unique resource name from "awcli-gateway-b" and store it as "gatewayHandleB"
    And I register a gateway "${CTX:gatewayHandleB}" on the control plane
    And the CLI is configured for the AI Workspace as "admin"

  @llm-provider
  Scenario: Publish and update an LLM provider through the CLI
    Given I generate a unique resource name from "awcli-provider" and store it as "artifactId"
    And I generate a unique API context from "/awcli-provider" and store it as "artifactContext"
    When the "llm-provider" project artifact is initialized
    And I build the "llm-provider" artifact
    And I apply the "llm-provider" artifact
    Then the CLI reports the "llm-provider" artifact was created
    And the "llm-provider" artifact is retrievable from the AI Workspace
    And the "llm-provider" artifact is listed in the AI Workspace
    When I edit the "llm-provider" artifact
    And I build the "llm-provider" artifact
    And I re-apply the "llm-provider" artifact
    Then the CLI reports the "llm-provider" artifact was updated
    And the "llm-provider" artifact is associated with gateway "${CTX:gatewayHandleA}"

  @llm-proxy
  Scenario: Publish and update an LLM proxy through the CLI
    Given I generate a unique resource name from "awcli-provider" and store it as "providerId"
    And I generate a unique API context from "/awcli-provider" and store it as "providerContext"
    And I create an LLM provider "${CTX:providerId}" via the control plane with context "${CTX:providerContext}" referencing template "openai"
    And I generate a unique resource name from "awcli-proxy" and store it as "artifactId"
    And I generate a unique API context from "/awcli-proxy" and store it as "artifactContext"
    When the "llm-proxy" project artifact is initialized
    And I build the "llm-proxy" artifact
    And I apply the "llm-proxy" artifact
    Then the CLI reports the "llm-proxy" artifact was created
    And the "llm-proxy" artifact is retrievable from the AI Workspace
    And the "llm-proxy" artifact is listed in the AI Workspace
    When I edit the "llm-proxy" artifact
    And I build the "llm-proxy" artifact
    And I re-apply the "llm-proxy" artifact
    Then the CLI reports the "llm-proxy" artifact was updated
    And the "llm-proxy" artifact is associated with gateway "${CTX:gatewayHandleA}"

  @mcp-proxy
  Scenario: Publish and update an MCP proxy through the CLI
    Given I generate a unique resource name from "awcli-mcp" and store it as "artifactId"
    And I generate a unique API context from "/awcli-mcp" and store it as "artifactContext"
    When the "mcp-proxy" project artifact is initialized
    And I build the "mcp-proxy" artifact
    And I apply the "mcp-proxy" artifact
    Then the CLI reports the "mcp-proxy" artifact was created
    And the "mcp-proxy" artifact is retrievable from the AI Workspace
    And the "mcp-proxy" artifact is listed in the AI Workspace
    When I edit the "mcp-proxy" artifact
    And I build the "mcp-proxy" artifact
    And I re-apply the "mcp-proxy" artifact
    Then the CLI reports the "mcp-proxy" artifact was updated
    And the "mcp-proxy" artifact is associated with gateway "${CTX:gatewayHandleA}"
