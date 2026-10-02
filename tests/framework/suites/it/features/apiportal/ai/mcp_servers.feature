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

Feature: API Portal MCP server management

  Scenario: A publisher creates and retrieves an MCP server
    Given I generate a unique resource name from "portal-mcp" and store it as "mcpId"
    And I send an authenticated API Portal "POST" multipart request to "/mcp-servers" as "publisher" with metadata and definition "mcp":
      """
      {"id": "${CTX:mcpId}", "name": "Test MCP Server", "version": "v1.0", "type": "MCP", "status": "PUBLISHED", "endPoints": {"productionURL": "https://backend.example.invalid", "sandboxURL": "https://sandbox.example.invalid"}}
      """
    Then the response status code should be 201
    And I store the JSON response field "id" as "mcpId"
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:mcpId}" as "publisher"
    Then the response status code should be 200
    And the JSON response field "type" should be "Mcp"

  Scenario: MCP creation rejects a missing type
    When I generate a unique resource name from "portal-mcp" and store it as "mcpId"
    And I send an authenticated API Portal "POST" multipart request to "/mcp-servers" as "publisher" with metadata and definition "mcp":
      """
      {"id": "${CTX:mcpId}", "name": "Missing Type MCP", "version": "v1.0", "status": "PUBLISHED", "endPoints": {"productionURL": "https://backend.example.invalid", "sandboxURL": "https://sandbox.example.invalid"}}
      """
    Then the response status code should be 400

  Scenario: MCP creation rejects a REST type
    When I generate a unique resource name from "portal-mcp" and store it as "mcpId"
    And I send an authenticated API Portal "POST" multipart request to "/mcp-servers" as "publisher" with metadata and definition "mcp":
      """
      {"id": "${CTX:mcpId}", "name": "Wrong Type MCP", "version": "v1.0", "type": "REST", "status": "PUBLISHED", "endPoints": {"productionURL": "https://backend.example.invalid", "sandboxURL": "https://sandbox.example.invalid"}}
      """
    Then the response status code should be 400

  Scenario: MCP creation rejects a missing definition
    Given I generate a unique resource name from "portal-mcp-no-definition" and store it as "mcpId"
    When I send an authenticated API Portal "POST" multipart request to "/mcp-servers" as "publisher" with metadata only:
      """
      {"id":"${CTX:mcpId}","name":"Missing Definition","version":"v1.0","type":"MCP","status":"PUBLISHED","endPoints":{"productionURL":"https://mcp.invalid","sandboxURL":"https://mcp.invalid"}}
      """
    Then the response status code should be 400

  Scenario: A publisher updates an MCP server
    Given I generate a unique resource name from "portal-mcp" and store it as "mcpId"
    And I send an authenticated API Portal "POST" multipart request to "/mcp-servers" as "publisher" with metadata and definition "mcp":
      """
      {"id": "${CTX:mcpId}", "name": "Original MCP", "version": "v1.0", "type": "MCP", "status": "PUBLISHED", "endPoints": {"productionURL": "https://backend.example.invalid", "sandboxURL": "https://sandbox.example.invalid"}}
      """
    Then the response status code should be 201
    When I send an authenticated API Portal "PUT" multipart request to "/mcp-servers/${CTX:mcpId}" as "publisher" with metadata and definition "mcp":
      """
      {"name": "Updated MCP", "version": "v1.0", "type": "MCP", "status": "PUBLISHED", "endPoints": {"productionURL": "https://updated.example.invalid", "sandboxURL": "https://sandbox.example.invalid"}}
      """
    Then the response status code should be 200
    And the JSON response field "name" should be "Updated MCP"

  Scenario: MCP update rejects a non-MCP type
    Given I generate a unique resource name from "portal-mcp" and store it as "mcpId"
    And I send an authenticated API Portal "POST" multipart request to "/mcp-servers" as "publisher" with metadata and definition "mcp":
      """
      {"id": "${CTX:mcpId}", "name": "Original MCP", "version": "v1.0", "type": "MCP", "status": "PUBLISHED", "endPoints": {"productionURL": "https://backend.example.invalid", "sandboxURL": "https://sandbox.example.invalid"}}
      """
    Then the response status code should be 201
    When I send an authenticated API Portal "PUT" multipart request to "/mcp-servers/${CTX:mcpId}" as "publisher" with metadata and definition "mcp":
      """
      {"name": "Invalid MCP", "version": "v1.0", "type": "REST", "status": "PUBLISHED", "endPoints": {"productionURL": "https://updated.example.invalid", "sandboxURL": "https://sandbox.example.invalid"}}
      """
    Then the response status code should be 400

  Scenario: MCP update rejects a missing type
    Given I generate a unique resource name from "portal-mcp-missing-type" and store it as "mcpId"
    And a unique API Portal multipart resource is created at "/mcp-servers" as "publisher" with metadata and definition "mcp" stored as "mcpId":
      """
      {"id":"${CTX:mcpId}","name":"Missing Type MCP","version":"v1.0","type":"MCP","status":"PUBLISHED","endPoints":{"productionURL":"https://mcp.invalid","sandboxURL":"https://mcp.invalid"}}
      """
    When I send an authenticated API Portal "PUT" multipart request to "/mcp-servers/${CTX:mcpId}" as "publisher" with metadata and definition "mcp":
      """
      {"name":"Missing Type MCP","version":"v1.0","status":"PUBLISHED","endPoints":{"productionURL":"https://mcp.invalid","sandboxURL":"https://mcp.invalid"}}
      """
    Then the response status code should be 400

  Scenario: A publisher deletes an MCP server
    Given I generate a unique resource name from "portal-mcp" and store it as "mcpId"
    And I send an authenticated API Portal "POST" multipart request to "/mcp-servers" as "publisher" with metadata and definition "mcp":
      """
      {"id": "${CTX:mcpId}", "name": "To Delete MCP", "version": "v1.0", "type": "MCP", "status": "PUBLISHED", "endPoints": {"productionURL": "https://backend.example.invalid", "sandboxURL": "https://sandbox.example.invalid"}}
      """
    Then the response status code should be 201
    When I send an authenticated API Portal "DELETE" request to "/mcp-servers/${CTX:mcpId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:mcpId}" as "publisher" until status 404
    Then the response status code should be 404

  Scenario: A publisher lists MCP servers
    Given I generate a unique resource name from "portal-mcp" and store it as "mcpId"
    And I send an authenticated API Portal "POST" multipart request to "/mcp-servers" as "publisher" with metadata and definition "mcp":
      """
      {"id": "${CTX:mcpId}", "name": "Listable MCP", "version": "v1.0", "type": "MCP", "status": "PUBLISHED", "endPoints": {"productionURL": "https://backend.example.invalid", "sandboxURL": "https://sandbox.example.invalid"}}
      """
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/mcp-servers" as "publisher"
    Then the response status code should be 200
    And the response body should contain "${CTX:mcpId}"

  Scenario: MCP creation rejects the wrong endpoint family
    Given I generate a unique resource name from "portal-mcp-wrong-endpoint" and store it as "mcpId"
    When I send an authenticated API Portal "POST" multipart request to "/apis" as "publisher" with metadata and definition "rest":
      """
      {"id":"${CTX:mcpId}","name":"Wrong Endpoint MCP","version":"v1.0","type":"MCP","status":"PUBLISHED","endPoints":{"productionURL":"https://mcp.invalid","sandboxURL":"https://mcp.invalid"}}
      """
    Then the response status code should be 400

  Scenario: A publisher generates an API key for an MCP server
    Given I generate a unique resource name from "portal-mcp-key" and store it as "mcpId"
    And a unique API Portal multipart resource is created at "/mcp-servers" as "publisher" with metadata and definition "mcp" stored as "mcpId":
      """
      {"id":"${CTX:mcpId}","name":"Key MCP ${CTX:mcpId}","version":"v1.0","type":"MCP","status":"PUBLISHED","endPoints":{"productionURL":"https://mcp.invalid","sandboxURL":"https://mcp.invalid"}}
      """
    When a publisher API key for MCP server "${CTX:mcpId}" is generated and stored as "keyId"
    Then the response status code should be 201
    And the JSON response should have field "key"

  Scenario: MCP server schema is stored as an asset
    Given I generate a unique resource name from "portal-mcp-schema" and store it as "mcpId"
    And a unique API Portal multipart resource is created at "/mcp-servers" as "publisher" with metadata and definition "mcp" stored as "mcpId":
      """
      {"id":"${CTX:mcpId}","name":"Schema MCP ${CTX:mcpId}","version":"v1.0","type":"MCP","status":"PUBLISHED","endPoints":{"productionURL":"https://mcp.invalid","sandboxURL":"https://mcp.invalid"}}
      """
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:mcpId}/assets?type=SCHEMA_DEFINITION&fileName=definition.yaml" as "publisher"
    Then the response status code should be 200
    And the response body should contain "name: ping"

  Scenario: API and MCP handles remain isolated
    Given a REST API is created in the API Portal and stored as "apiId"
    And I generate a unique resource name from "portal-mcp-isolation" and store it as "mcpId"
    And a unique API Portal multipart resource is created at "/mcp-servers" as "publisher" with metadata and definition "mcp" stored as "mcpId":
      """
      {"id":"${CTX:mcpId}","name":"Isolation MCP ${CTX:mcpId}","version":"v1.0","type":"MCP","status":"PUBLISHED","endPoints":{"productionURL":"https://mcp.invalid","sandboxURL":"https://mcp.invalid"}}
      """
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:apiId}" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:mcpId}" as "publisher"
    Then the response status code should be 404

  Scenario: An MCP server exposes its tools through the registry
    Given I generate a unique resource name from "portal-registry-mcp" and store it as "mcpId"
    And a unique API Portal multipart resource is created at "/mcp-servers" as "publisher" with metadata and definition "mcp" stored as "mcpId":
      """
      {"id":"${CTX:mcpId}","name":"Registry MCP ${CTX:mcpId}","version":"v1.0","type":"MCP","status":"PUBLISHED","endPoints":{"productionURL":"https://mcp.invalid","sandboxURL":"https://mcp.invalid"}}
      """
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers/Registry%20MCP%20${CTX:mcpId}/versions/v1.0"
    Then the response status code should be 200
    And the response body should contain "ping"

  Scenario: A plain REST API is not resolved by the MCP resource endpoint
    Given a REST API is created in the API Portal and stored as "apiId"
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:apiId}" as "publisher"
    Then the response status code should be 404

  Scenario: An MCP server is not resolved by the REST API endpoint
    Given I generate a unique resource name from "portal-mcp-cross-api" and store it as "mcpId"
    And a unique API Portal multipart resource is created at "/mcp-servers" as "publisher" with metadata and definition "mcp" stored as "mcpId":
      """
      {"id":"${CTX:mcpId}","name":"Cross API MCP ${CTX:mcpId}","version":"v1.0","type":"MCP","status":"PUBLISHED","endPoints":{"productionURL":"https://mcp.invalid","sandboxURL":"https://mcp.invalid"}}
      """
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:mcpId}" as "publisher"
    Then the response status code should be 404

  Scenario: Plain REST APIs are excluded from the MCP list
    Given I generate a unique resource name from "portal-rest-outside-mcp" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "rest" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"REST Outside MCP ${CTX:apiId}","version":"v1.0","type":"REST","status":"PUBLISHED","endPoints":{"productionURL":"https://api.invalid","sandboxURL":"https://api.invalid"}}
      """
    When I send an authenticated API Portal "GET" request to "/mcp-servers?name=REST%20Outside%20MCP%20${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    And the response body should not contain "${CTX:apiId}"

  Scenario: MCP servers are excluded from the REST API list
    Given I generate a unique resource name from "portal-mcp-outside-rest" and store it as "mcpId"
    And a unique API Portal multipart resource is created at "/mcp-servers" as "publisher" with metadata and definition "mcp" stored as "mcpId":
      """
      {"id":"${CTX:mcpId}","name":"MCP Outside REST ${CTX:mcpId}","version":"v1.0","type":"MCP","status":"PUBLISHED","endPoints":{"productionURL":"https://mcp.invalid","sandboxURL":"https://mcp.invalid"}}
      """
    When I send an authenticated API Portal "GET" request to "/apis?name=MCP%20Outside%20REST%20${CTX:mcpId}" as "publisher"
    Then the response status code should be 200
    And the response body should not contain "${CTX:mcpId}"

  Scenario: The REST API key endpoint rejects an MCP handle
    Given I generate a unique resource name from "portal-mcp-api-key-cross" and store it as "mcpId"
    And a unique API Portal multipart resource is created at "/mcp-servers" as "publisher" with metadata and definition "mcp" stored as "mcpId":
      """
      {"id":"${CTX:mcpId}","name":"MCP Key Cross ${CTX:mcpId}","version":"v1.0","type":"MCP","status":"PUBLISHED","endPoints":{"productionURL":"https://mcp.invalid","sandboxURL":"https://mcp.invalid"}}
      """
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:mcpId}/api-keys" as "publisher"
    Then the response status code should be 404

  Scenario: The MCP API key endpoint rejects a REST handle
    Given a REST API is created in the API Portal and stored as "apiId"
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:apiId}/api-keys" as "publisher"
    Then the response status code should be 404
