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

@NETAC
Feature: The control plane never connects to a destination its network access control refuses
  As a platform operator
  I want every URL a tenant or user gives the control plane checked before it is used
  So that no request can make the control plane reach the cloud metadata service or another refused host

  # The block's platform-api runs network access control at its defaults: every destination is
  # allowed except link-local (cloud metadata), unspecified and multicast addresses, which are
  # always refused. A refusal happens before any request is sent, and answers 400
  # DESTINATION_NOT_ALLOWED with a message that never names the host or the reason.
  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique resource name from "netac-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  @NETAC-PA-001 @type:negative
  Scenario: Fetching MCP server info from the cloud metadata address is refused, while the testbench MCP server answers
    When I send a "POST" request to the control plane at "/mcp-proxies/fetch-server-info" with body:
      """
      {"url": "http://169.254.169.254/latest/meta-data/"}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"
    And the JSON response field "message" should be "The URL's destination is not permitted."
    And the response body should not contain "169.254"
    When I send a "POST" request to the control plane at "/mcp-proxies/fetch-server-info" with body:
      """
      {"url": "http://testbench:3009/mcp"}
      """
    Then the response status code should be 200
    And the JSON response should have field "serverInfo"

  @NETAC-PA-002 @type:negative
  Scenario: Previewing an Agent Card from the cloud metadata address is refused, while a reachable agent's card previews
    Given I generate a unique resource name from "netac-card" and store it as "cardScope"
    And I store the Agent Card fixture URL for scope "${CTX:cardScope}" in mode "ok" as "okUrl"
    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"url": "http://169.254.169.254/.well-known/agent-card.json"}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"
    And the JSON response field "message" should be "The URL's destination is not permitted."
    And the response body should not contain "169.254"
    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"url": "${CTX:okUrl}"}
      """
    Then the response status code should be 200
    And the Agent Card fixture should have received 1 requests for scope "${CTX:cardScope}"

  @NETAC-PA-003 @type:negative
  Scenario: A stored Agent proxy whose upstream is refused gets the same 400 on every fetch, never a cached 503
    Given I generate a unique resource name from "netac-card-stored" and store it as "cardScope"
    And I generate a unique resource name from "netac-refused-agent" and store it as "refusedAgent"
    And I generate a unique resource name from "netac-allowed-agent" and store it as "allowedAgent"
    And I generate a unique API context from "/netac-refused" and store it as "refusedContext"
    And I generate a unique API context from "/netac-allowed" and store it as "allowedContext"
    And I store the Agent Card fixture URL for scope "${CTX:cardScope}" in mode "ok" as "okUrl"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:refusedAgent}                              |
      | displayName | Refused Card Agent                               |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:refusedContext}                            |
      | upstreamUrl | http://169.254.169.254                           |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    And the response status code should be 201
    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"agentProxyId": "${CTX:refusedAgent}"}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"
    And the response body should not contain "169.254"
    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"agentProxyId": "${CTX:refusedAgent}"}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"
    And the response header "Age" should not exist
    Given I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:allowedAgent}                              |
      | displayName | Allowed Card Agent                               |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:allowedContext}                            |
      | upstreamUrl | ${CTX:okUrl}                                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    And the response status code should be 201
    When I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"agentProxyId": "${CTX:allowedAgent}"}
      """
    Then the response status code should be 200
    And the Agent Card fixture should have received 1 requests for scope "${CTX:cardScope}"

  @NETAC-PA-004 @type:negative
  Scenario: Registering an API portal at the cloud metadata address is refused and creates nothing, while a permitted URL registers
    Given I generate a unique resource name from "netac-portal" and store it as "refusedPortal"
    And I generate a unique resource name from "netac-portal-ok" and store it as "allowedPortal"
    When I send a "POST" request to the control plane at "/api-portals" with body:
      """
      {"name": "${CTX:refusedPortal}", "handle": "${CTX:refusedPortal}", "url": "https://169.254.169.254/portal", "sharedKey": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"
    And the response body should not contain "169.254"
    When I send a "GET" request to the control plane at "/api-portals/${CTX:refusedPortal}"
    Then the response status code should be 404
    When I send a "POST" request to the control plane at "/api-portals" with body:
      """
      {"name": "${CTX:allowedPortal}", "handle": "${CTX:allowedPortal}", "url": "https://portal.example.com", "sharedKey": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
      """
    Then the response status code should be 201
    When I send a "GET" request to the control plane at "/api-portals/${CTX:allowedPortal}"
    Then the response status code should be 200

  @NETAC-PA-005 @type:negative
  Scenario: An LLM provider template whose spec URL is the cloud metadata address is refused on create and on update
    Given I generate a unique resource name from "netac-template" and store it as "refusedTemplate"
    And I generate a unique resource name from "netac-template-ok" and store it as "allowedTemplate"
    When I create an LLM provider template via the control plane from "resources/templates/llm-provider-template-cp.yaml" with values:
      | displayName    | ${CTX:refusedTemplate}                       |
      | endpointUrl    | https://llm.example.com/v1                   |
      | openapiSpecUrl | http://169.254.169.254/latest/openapi.yaml   |
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"
    And the response body should not contain "169.254"
    When I create an LLM provider template via the control plane from "resources/templates/llm-provider-template-cp.yaml" with values:
      | displayName    | ${CTX:allowedTemplate}                       |
      | endpointUrl    | https://llm.example.com/v1                   |
      | openapiSpecUrl | https://specs.example.com/llm/openapi.yaml   |
    Then the response status code should be 201
    And I store the JSON response field "id" as "templateId"
    When I update the LLM provider template "${CTX:templateId}" via the control plane from "resources/templates/llm-provider-template-cp.yaml" with values:
      | displayName    | ${CTX:allowedTemplate}                       |
      | endpointUrl    | https://llm.example.com/v1                   |
      | openapiSpecUrl | http://169.254.169.254/latest/openapi.yaml   |
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"
    When I send a "GET" request to the control plane at "/llm-provider-templates/${CTX:templateId}"
    Then the response status code should be 200
    And the JSON response field "metadata.openapiSpecUrl" should be "https://specs.example.com/llm/openapi.yaml"
