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

@gateway-controller-mcp
Feature: Gateway controller management MCP endpoint
  As an operator using an MCP client
  I want to manage the gateway through the controller's management MCP endpoint
  So that resources deployed through MCP are validated, routed and authorized like REST deployments

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: tools/list advertises the management tools
    When I send an MCP "tools/list" request to the "gateway-controller" service
    Then the response status code should be 200
    And the response header "Content-Type" should contain "text/event-stream"
    And the response should be valid JSON
    And the JSON response array field "result.tools" should have 12 items
    And the response body should contain "wso2_apip_gw_deploy_api"
    And the response body should contain "wso2_apip_gw_undeploy_api"
    And the response body should contain "wso2_apip_gw_get_resource"

  Scenario: An API deployed through MCP routes traffic and is readable through MCP
    Given I generate a unique resource name from "mcp-tool-api" and store it as "apiName"
    And I generate a unique value from "mcp-tool-api" and store it as "apiDisplayName"
    And I generate a unique API version from "mcp-tool-api" and store it as "apiVersion"
    And I generate a unique API context from "/mcp-tool-api" and store it as "apiContext"
    When I deploy API through MCP from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiDisplayName}             |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/api/v2       |
      | spec.operations        | [{"method":"GET","path":"/{country_code}/{city}"}] |
    Then the response status code should be 200
    And the JSON response field "result.structuredContent.status" should be "success"
    And the JSON response field "result.structuredContent.operation" should be "create"
    And the JSON response field "result.structuredContent.kind" should be "RestApi"
    And the JSON response field "result.structuredContent.resource.metadata.name" should be "${CTX:apiName}"
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/us/seattle" until status 200
    Then the response should be successful
    When I call the MCP tool "wso2_apip_gw_get_resource" on the "gateway-controller" service with arguments:
      """
      {"kind":"RestApi","id":"${CTX:apiName}"}
      """
    Then the response status code should be 200
    And the JSON response field "result.isError" should not exist
    And the JSON response field "result.structuredContent.id" should be "${CTX:apiName}"
    And the JSON response field "result.structuredContent.resource.spec.displayName" should be "${CTX:apiDisplayName}"

  Scenario: An API updated through MCP serves the new operation
    Given I generate a unique resource name from "mcp-tool-update" and store it as "apiName"
    And I generate a unique value from "mcp-tool-update" and store it as "apiDisplayName"
    And I generate a unique API version from "mcp-tool-update" and store it as "apiVersion"
    And I generate a unique API context from "/mcp-tool-update" and store it as "apiContext"
    And I deploy API through MCP from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiDisplayName}             |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/api/v2       |
      | spec.operations        | [{"method":"GET","path":"/{country_code}/{city}"}] |
    And the JSON response field "result.structuredContent.status" should be "success"
    When I update API "${CTX:apiName}" through MCP from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiDisplayName}             |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/api/v2       |
      | spec.operations        | [{"method":"GET","path":"/{country_code}/{city}"},{"method":"GET","path":"/alerts/active"}] |
    Then the response status code should be 200
    And the JSON response field "result.structuredContent.status" should be "success"
    And the JSON response field "result.structuredContent.operation" should be "update"
    And the JSON response field "result.structuredContent.id" should be "${CTX:apiName}"
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/alerts/active" until status 200
    Then the response should be successful

  Scenario: An API undeployed through MCP stops routing and can no longer be read
    Given I generate a unique resource name from "mcp-tool-undeploy" and store it as "apiName"
    And I generate a unique value from "mcp-tool-undeploy" and store it as "apiDisplayName"
    And I generate a unique API version from "mcp-tool-undeploy" and store it as "apiVersion"
    And I generate a unique API context from "/mcp-tool-undeploy" and store it as "apiContext"
    And I deploy API through MCP from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiDisplayName}             |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/api/v2       |
      | spec.operations        | [{"method":"GET","path":"/{country_code}/{city}"}] |
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/us/seattle" until status 200
    When I undeploy API "${CTX:apiName}" through MCP
    Then the response status code should be 200
    And the JSON response field "result.structuredContent.status" should be "success"
    And the JSON response field "result.structuredContent.message" should contain "deleted successfully"
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/us/seattle" until status 404
    Then the response status code should be 404
    When I call the MCP tool "wso2_apip_gw_get_resource" on the "gateway-controller" service with arguments:
      """
      {"kind":"RestApi","id":"${CTX:apiName}"}
      """
    Then the response status code should be 200
    And the JSON response field "result.isError" should be "true"
    And the JSON response field "result.content[0].text" should contain "not found"

  Scenario: An invalid manifest is returned as a tool error and nothing is deployed
    Given I generate a unique resource name from "mcp-tool-invalid" and store it as "apiName"
    And I generate a unique API context from "/mcp-tool-invalid" and store it as "apiContext"
    When I deploy API through MCP from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}         |
      | name                     | ${CTX:apiName}                    |
      | spec.displayName         | MCP-Invalid-Upstream-API          |
      | spec.version             | v1.0                              |
      | spec.context             | ${CTX:apiContext}/$version        |
      | spec.upstreamDefinitions | [{"name":"backend-default","basePath":"/api-main","upstreams":[{"url":"http://testbench:3000?region=eu"}]}] |
      | spec.upstream.main.ref   | backend-default                   |
      | spec.operations          | [{"method":"GET","path":"/endpoint"}] |
    Then the response status code should be 200
    And the JSON response field "result.isError" should be "true"
    And the JSON response field "result.content[0].text" should contain "failed to create RestApi: configuration validation failed"
    When I call the MCP tool "wso2_apip_gw_get_resource" on the "gateway-controller" service with arguments:
      """
      {"kind":"RestApi","id":"${CTX:apiName}"}
      """
    Then the JSON response field "result.isError" should be "true"
    And the JSON response field "result.content[0].text" should contain "not found"

  Scenario: A developer may deploy a REST API through MCP
    Given I authenticate using basic auth as "developer"
    And I generate a unique resource name from "mcp-tool-developer" and store it as "apiName"
    And I generate a unique value from "mcp-tool-developer" and store it as "apiDisplayName"
    And I generate a unique API version from "mcp-tool-developer" and store it as "apiVersion"
    And I generate a unique API context from "/mcp-tool-developer" and store it as "apiContext"
    When I deploy API through MCP from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiDisplayName}             |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/api/v2       |
      | spec.operations        | [{"method":"GET","path":"/{country_code}/{city}"}] |
    Then the response status code should be 200
    And the JSON response field "result.structuredContent.status" should be "success"

  Scenario: A consumer is refused a REST API deploy at the HTTP layer with a step-up challenge
    Given I authenticate using basic auth as "consumer"
    And I generate a unique resource name from "mcp-tool-consumer" and store it as "apiName"
    And I generate a unique API context from "/mcp-tool-consumer" and store it as "apiContext"
    When I deploy API through MCP from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | MCP-Consumer-Denied-API           |
      | spec.version           | v1.0                              |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/api/v2       |
      | spec.operations        | [{"method":"GET","path":"/{country_code}/{city}"}] |
    Then the response status code should be 403
    And the response header "WWW-Authenticate" should match pattern "^Bearer error=.insufficient_scope., .*scope=.admin developer.$"
    And the JSON response field "code" should be "insufficient_scope"

  Scenario: A request without credentials is rejected before reaching MCP
    Given I clear all headers
    When I send an MCP "tools/list" request to the "gateway-controller" service
    Then the response status code should be 401
    And the JSON response field "code" should be "skip_authz"
    And the JSON response field "message" should be "no valid authentication credentials provided"

  Scenario: A body that is not a single JSON-RPC message is rejected
    When I send a "POST" request to the "gateway-controller" service at "/mcp" with body:
      """
      {"jsonrpc":
      """
    Then the response status code should be 400
    And the JSON response field "message" should be "Request body must be a single JSON-RPC message."
