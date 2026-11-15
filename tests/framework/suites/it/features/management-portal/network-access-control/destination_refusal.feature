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
  # always refused. A refusal happens before any request is sent or anything is stored, and
  # answers 400 DESTINATION_NOT_ALLOWED with a message that never names the host or the reason.
  # It covers the URLs platform-api fetches and every upstream URL an artifact's create, update
  # or deploy request carries; an API portal's URL is deployment configuration and is not checked.
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

  # The refused proxy's upstream is an allowed host that redirects to the cloud metadata address:
  # a create naming that address directly is itself refused (NETAC-PA-013).
  @NETAC-PA-003 @type:negative
  Scenario: A stored Agent proxy whose upstream is refused gets the same 400 on every fetch, never a cached 503
    Given I generate a unique resource name from "netac-card-stored" and store it as "cardScope"
    And I generate a unique resource name from "netac-card-redirect" and store it as "redirectScope"
    And I generate a unique resource name from "netac-refused-agent" and store it as "refusedAgent"
    And I generate a unique resource name from "netac-allowed-agent" and store it as "allowedAgent"
    And I generate a unique API context from "/netac-refused" and store it as "refusedContext"
    And I generate a unique API context from "/netac-allowed" and store it as "allowedContext"
    And I store the Agent Card fixture URL for scope "${CTX:cardScope}" in mode "ok" as "okUrl"
    And I store the Agent Card fixture URL for scope "${CTX:redirectScope}" in mode "redirect-metadata" as "redirectUrl"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:refusedAgent}                              |
      | displayName | Refused Card Agent                               |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:refusedContext}                            |
      | upstreamUrl | ${CTX:redirectUrl}                               |
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
    And the Agent Card fixture should have received 2 requests for scope "${CTX:redirectScope}"
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

  @NETAC-PA-011 @type:negative
  Scenario: A REST API whose upstream is the cloud metadata address is refused on create and on update, while a permitted upstream is created
    Given I generate a unique resource name from "netac-rest" and store it as "refusedApi"
    And I generate a unique resource name from "netac-rest-ok" and store it as "allowedApi"
    And I generate a unique API context from "/netac-rest" and store it as "refusedContext"
    And I generate a unique API context from "/netac-rest-ok" and store it as "allowedContext"
    When I create a REST API via the control plane from "resources/templates/rest-api-cp.yaml" with values:
      | id          | ${CTX:refusedApi}             |
      | displayName | Refused REST API              |
      | projectId   | ${CTX:projectHandle}          |
      | context     | ${CTX:refusedContext}         |
      | upstreamUrl | http://169.254.169.254/latest |
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"
    And the JSON response field "errors[0].field" should be "upstream.main.url"
    And the response body should not contain "169.254"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:refusedApi}"
    Then the response status code should be 404
    When I create a REST API via the control plane from "resources/templates/rest-api-cp.yaml" with values:
      | id          | ${CTX:allowedApi}     |
      | displayName | Allowed REST API      |
      | projectId   | ${CTX:projectHandle}  |
      | context     | ${CTX:allowedContext} |
      | upstreamUrl | http://testbench:3000 |
    Then the response status code should be 201
    When I update the REST API "${CTX:allowedApi}" via the control plane from "resources/templates/rest-api-cp.yaml" with values:
      | id          | ${CTX:allowedApi}             |
      | displayName | Allowed REST API              |
      | projectId   | ${CTX:projectHandle}          |
      | context     | ${CTX:allowedContext}         |
      | upstreamUrl | http://169.254.169.254/latest |
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"
    And the JSON response field "errors[0].field" should be "upstream.main.url"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:allowedApi}"
    Then the response status code should be 200
    And the JSON response field "upstream.main.url" should be "http://testbench:3000"

  @NETAC-PA-012 @type:negative
  Scenario: An LLM provider whose upstream is the cloud metadata address is refused, while a permitted upstream is created
    Given I generate a unique value from "NETAC refused provider" and store it as "refusedProvider"
    And I generate a unique value from "NETAC allowed provider" and store it as "allowedProvider"
    When I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName | ${CTX:refusedProvider}           |
      | template    | openai                           |
      | upstreamUrl | http://169.254.169.254/openai/v1 |
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"
    And the JSON response field "errors[0].field" should be "upstream.main.url"
    And the response body should not contain "169.254"
    When I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName | ${CTX:allowedProvider}          |
      | template    | openai                          |
      | upstreamUrl | http://testbench:3008/openai/v1 |
    Then the response status code should be 201

  @NETAC-PA-013 @type:negative
  Scenario: An Agent proxy whose upstream is the cloud metadata address is refused and not created, while a permitted upstream is created
    Given I generate a unique resource name from "netac-card-create" and store it as "cardScope"
    And I generate a unique resource name from "netac-agent-refused" and store it as "refusedAgent"
    And I generate a unique resource name from "netac-agent-allowed" and store it as "allowedAgent"
    And I generate a unique API context from "/netac-agent-refused" and store it as "refusedContext"
    And I generate a unique API context from "/netac-agent-allowed" and store it as "allowedContext"
    And I store the Agent Card fixture URL for scope "${CTX:cardScope}" in mode "ok" as "okUrl"
    When I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:refusedAgent}                              |
      | displayName | Refused Agent                                    |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:refusedContext}                            |
      | upstreamUrl | http://169.254.169.254                           |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"
    And the JSON response field "errors[0].field" should be "upstream.main.url"
    And the response body should not contain "169.254"
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:refusedAgent}"
    Then the response status code should be 404
    When I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:allowedAgent}                              |
      | displayName | Allowed Agent                                    |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:allowedContext}                            |
      | upstreamUrl | ${CTX:okUrl}                                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    Then the response status code should be 201

  @NETAC-PA-014 @type:negative
  Scenario: Deploying a REST API with an endpoint override at the cloud metadata address is refused, while a permitted override deploys
    Given I generate a unique resource name from "netac-deploy" and store it as "api"
    And I generate a unique API context from "/netac-deploy" and store it as "apiContext"
    And I store the registered gateway id as "gatewayId"
    And I create a REST API "${CTX:api}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}"
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:api}/deployments" with body:
      """
      {"name": "${UNIQUE:netac-refused}", "base": "current", "gatewayId": "${CTX:gatewayId}", "metadata": {"endpointUrl": "http://169.254.169.254/latest"}}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"
    And the JSON response field "errors[0].field" should be "metadata.endpointUrl"
    And the response body should not contain "169.254"
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:api}/deployments" with body:
      """
      {"name": "${UNIQUE:netac-allowed}", "base": "current", "gatewayId": "${CTX:gatewayId}", "metadata": {"endpointUrl": "http://testbench:3000"}}
      """
    Then the response status code should be 201
    And I store the JSON response field "deploymentId" as "deploymentId"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    Then the JSON response field "status" should be "DEPLOYED"

  # An API portal's URL is deployment configuration, not a tenant destination, so access control
  # does not apply to it. The REST API named at the same address shows the rule is active.
  @NETAC-PA-015
  Scenario: Registering an API portal at an address access control refuses succeeds, while a REST API upstream at that address is refused
    Given I generate a unique resource name from "netac-portal" and store it as "portal"
    And I generate a unique resource name from "netac-portal-api" and store it as "api"
    And I generate a unique API context from "/netac-portal-api" and store it as "apiContext"
    When I send a "POST" request to the control plane at "/api-portals" with body:
      """
      {"name": "${CTX:portal}", "handle": "${CTX:portal}", "url": "https://169.254.169.254/portal", "sharedKey": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
      """
    Then the response status code should be 201
    When I send a "GET" request to the control plane at "/api-portals/${CTX:portal}"
    Then the response status code should be 200
    And the JSON response field "url" should be "https://169.254.169.254/portal"
    When I create a REST API via the control plane from "resources/templates/rest-api-cp.yaml" with values:
      | id          | ${CTX:api}                     |
      | displayName | Portal Address REST API        |
      | projectId   | ${CTX:projectHandle}           |
      | context     | ${CTX:apiContext}              |
      | upstreamUrl | https://169.254.169.254/portal |
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"

  @NETAC-PA-016 @type:negative
  Scenario: A GraphQL API whose SDL URL is the cloud metadata address is refused before the SDL is fetched, while one with an inline SDL is created
    Given I generate a unique resource name from "netac-gql" and store it as "refusedApi"
    And I generate a unique resource name from "netac-gql-ok" and store it as "allowedApi"
    And I generate a unique API context from "/netac-gql" and store it as "refusedContext"
    And I generate a unique API context from "/netac-gql-ok" and store it as "allowedContext"
    When I send a multipart "POST" request to the control plane at "/graphql-apis" with fields:
      | metadata | {"id": "${CTX:refusedApi}", "displayName": "${CTX:refusedApi}", "context": "${CTX:refusedContext}", "version": "v1", "projectId": "${CTX:projectHandle}", "upstream": {"main": {"url": "http://testbench:3017/graphql"}}, "schemaSource": "url", "sdlUrl": "http://169.254.169.254/schema.graphql"} |
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"
    And the JSON response field "errors[0].field" should be "sdlUrl"
    And the response body should not contain "169.254"
    When I send a "GET" request to the control plane at "/graphql-apis/${CTX:refusedApi}"
    Then the response status code should be 404
    When I send a multipart "POST" request to the control plane at "/graphql-apis" with fields:
      | metadata | {"id": "${CTX:allowedApi}", "displayName": "${CTX:allowedApi}", "context": "${CTX:allowedContext}", "version": "v1", "projectId": "${CTX:projectHandle}", "upstream": {"main": {"url": "http://testbench:3017/graphql"}}, "schemaSource": "inline", "sdl": "type Query { hello: String }"} |
    Then the response status code should be 201

  @NETAC-PA-017 @type:negative
  Scenario: An MCP proxy whose per-gateway endpoint override is the cloud metadata address is refused, while a permitted override is created
    Given I generate a unique resource name from "netac-mcp" and store it as "refusedProxy"
    And I generate a unique resource name from "netac-mcp-ok" and store it as "allowedProxy"
    And I generate a unique API context from "/netac-mcp" and store it as "refusedContext"
    And I generate a unique API context from "/netac-mcp-ok" and store it as "allowedContext"
    And I store the registered gateway id as "gatewayId"
    When I send a "POST" request to the control plane at "/mcp-proxies" with body:
      """
      {"id": "${CTX:refusedProxy}", "displayName": "${CTX:refusedProxy}", "version": "v1.0", "context": "${CTX:refusedContext}", "projectId": "${CTX:projectHandle}", "mcpSpecVersions": ["2025-06-18"], "upstream": {"main": {"url": "http://testbench:3009/mcp"}}, "associatedGateways": [{"id": "${CTX:gatewayId}", "configurations": {"endpointUrl": "http://169.254.169.254/mcp"}}]}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"
    And the JSON response field "errors[0].field" should be "associatedGateways[0].configurations.endpointUrl"
    And the response body should not contain "169.254"
    When I send a "GET" request to the control plane at "/mcp-proxies/${CTX:refusedProxy}"
    Then the response status code should be 404
    When I send a "POST" request to the control plane at "/mcp-proxies" with body:
      """
      {"id": "${CTX:allowedProxy}", "displayName": "${CTX:allowedProxy}", "version": "v1.0", "context": "${CTX:allowedContext}", "projectId": "${CTX:projectHandle}", "mcpSpecVersions": ["2025-06-18"], "upstream": {"main": {"url": "http://testbench:3009/mcp"}}, "associatedGateways": [{"id": "${CTX:gatewayId}", "configurations": {"endpointUrl": "http://testbench:3009/mcp"}}]}
      """
    Then the response status code should be 201

  # Schema validation is a dry run, but a refused source is still an error the caller must
  # correct, not the "could not derive a schema" outcome an unreachable source gets.
  @NETAC-PA-020 @type:negative
  Scenario: Validating a GraphQL schema from a refused SDL URL or introspection endpoint is refused, while a permitted SDL URL resolves
    When I send a multipart "POST" request to the control plane at "/graphql-apis/validate-schema" with fields:
      | metadata | {"schemaSource": "url", "sdlUrl": "http://169.254.169.254/schema.graphql"} |
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"
    And the JSON response field "errors[0].field" should be "sdlUrl"
    And the response body should not contain "169.254"
    When I send a multipart "POST" request to the control plane at "/graphql-apis/validate-schema" with fields:
      | metadata | {"schemaSource": "introspection", "upstream": {"main": {"url": "http://169.254.169.254/graphql"}}} |
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"
    And the JSON response field "errors[0].field" should be "upstream.main.url"
    When I send a multipart "POST" request to the control plane at "/graphql-apis/validate-schema" with fields:
      | metadata | {"schemaSource": "url", "sdlUrl": "http://testbench:3024/specs/hello.graphql"} |
    Then the response status code should be 200
    And the JSON response field "resolved" should be "true"

