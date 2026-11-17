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


@APIPORTAL @apiportal-mcp-server
Feature: MCP servers are managed through the API Portal REST API and its MCP registry
  As an API publisher
  I want to publish MCP servers with their tools, through the portal's API or its MCP registry
  So that developers and AI agents find each server, and nothing else is mistaken for one

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml) at the API level. MCP
  # servers are rendered from resources/templates/api-portal-api.yaml with type MCP and sent to
  # /mcp-servers; the "mcp" definition holds a single "ping" tool. REST APIs and MCP servers live in
  # separate collections, so each handle resolves only in its own. Registry servers are named
  # "<namespace>/<name>", and the name after the slash is the server's handle in the portal.

  @APIP-017 @type:smoke
  Scenario: A publisher manages an MCP server, which carries the AI Ready badge while agents may see it
    # A publisher manages an MCP server from creation to deletion
    Given I generate a unique resource name from "portal-mcp" and store it as "mcpId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" at "/mcp-servers" as "publisher" with definition "mcp" with values:
      | id   | ${CTX:mcpId} |
      | name | ${CTX:mcpId} |
      | type | MCP |
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:mcpId}" as "publisher"
    Then the response status code should be 200
    And the JSON response field "type" should be "Mcp"
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:mcpId}/assets?type=SCHEMA_DEFINITION&fileName=definition.yaml" as "publisher"
    Then the response status code should be 200
    And the response body should contain "name: ping"
    When I send an authenticated API Portal "GET" request to "/mcp-servers" as "publisher"
    Then the response body should contain "${CTX:mcpId}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers/${CTX:mcpId}/versions/v1.0"
    Then the response status code should be 200
    And the response body should contain "ping"

    When I update the API Portal API "${CTX:mcpId}" from "resources/templates/api-portal-api.yaml" at "/mcp-servers" as "publisher" with definition "mcp" with values:
      | id   | ${CTX:mcpId}         |
      | name | ${CTX:mcpId}-renamed |
      | type | MCP                  |
    Then the response status code should be 200
    And the JSON response field "name" should be "${CTX:mcpId}-renamed"

    When a publisher API key for MCP server "${CTX:mcpId}" is generated and stored as "keyId"
    Then the response status code should be 201
    And the JSON response should have field "key"
    When I send an authenticated API Portal "DELETE" request to "/mcp-servers/${CTX:mcpId}" as "publisher"
    Then the response status code should be 409
    When I send an authenticated API Portal "POST" request to "/mcp-servers/${CTX:mcpId}/api-keys/revoke" as "publisher" with JSON values:
      | keyId | ${CTX:keyId} |
    Then the response status code should be 204
    When I send an authenticated API Portal "DELETE" request to "/mcp-servers/${CTX:mcpId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:mcpId}" as "publisher" until status 404
    Then the response status code should be 404

    # An MCP server carries the AI Ready badge only while agents may see it
    Given I generate a unique resource name from "portal-ai-ready-label" and store it as "labelId"
    And I generate a unique resource name from "portal-ai-ready-view" and store it as "viewId"
    And I generate a unique resource name from "portal-ai-hidden-label" and store it as "hiddenLabelId"
    And I generate a unique resource name from "portal-ai-hidden-view" and store it as "hiddenViewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | AI Ready Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | AI Ready View      |
      | labels      | ["${CTX:labelId}"] |
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "hiddenLabelId":
      | id          | ${CTX:hiddenLabelId} |
      | displayName | AI Hidden Label      |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "hiddenViewId":
      | id          | ${CTX:hiddenViewId}      |
      | displayName | AI Hidden View           |
      | labels      | ["${CTX:hiddenLabelId}"] |
    And I generate a unique resource name from "portal-ai-ready-mcp" and store it as "mcpId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" at "/mcp-servers" as "publisher" with definition "mcp" with values:
      | id              | ${CTX:mcpId}       |
      | name            | ${CTX:mcpId}       |
      | type            | MCP                |
      | labels          | ["${CTX:labelId}"] |
      | agentVisibility | VISIBLE            |
    And the response status code should be 201
    And I generate a unique resource name from "portal-ai-hidden-mcp" and store it as "hiddenMcpId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" at "/mcp-servers" as "publisher" with definition "mcp" with values:
      | id              | ${CTX:hiddenMcpId}       |
      | name            | ${CTX:hiddenMcpId}       |
      | type            | MCP                      |
      | labels          | ["${CTX:hiddenLabelId}"] |
      | agentVisibility | HIDDEN                   |
    And the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/mcps"
    Then the response status code should be 200
    And the response body should contain "${CTX:mcpId}"
    And the response body should contain "AI Ready"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/mcp/${CTX:mcpId}"
    Then the response status code should be 200
    And the response body should contain "AI Ready"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/llms.txt"
    Then the response status code should be 200
    And the response body should contain "## MCPs"
    And the response body should contain "${CTX:mcpId}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:hiddenViewId}/mcps"
    Then the response status code should be 200
    And the response body should contain "${CTX:hiddenMcpId}"
    And the response body should not contain "AI Ready"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:hiddenViewId}/mcp/${CTX:hiddenMcpId}"
    Then the response status code should be 200
    And the response body should not contain "AI Ready"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:hiddenViewId}/llms.txt"
    Then the response status code should be 200
    And the response body should not contain "${CTX:hiddenMcpId}"

  @APIP-017 @type:negative @type:security
  Scenario: Invalid MCP servers are refused, handles never cross with REST APIs, and one in use cannot be deleted
    # Creating an MCP server with no type is refused and stores nothing
    Given I generate a unique resource name from "portal-invalid-mcp" and store it as "mcpId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" at "/mcp-servers" as "publisher" with definition "mcp" with values:
      | id   | ${CTX:mcpId} |
      | name | ${CTX:mcpId} |
      | type | null       |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:mcpId}" as "publisher"
    Then the response status code should be 404
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" at "/mcp-servers" as "publisher" with definition "mcp" with values:
      | id   | ${CTX:mcpId} |
      | name | ${CTX:mcpId} |
      | type | MCP |
    Then the response status code should be 201

    # Creating an MCP server with the REST type is refused and stores nothing
    Given I generate a unique resource name from "portal-invalid-mcp" and store it as "mcpId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" at "/mcp-servers" as "publisher" with definition "mcp" with values:
      | id   | ${CTX:mcpId} |
      | name | ${CTX:mcpId} |
      | type | REST       |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:mcpId}" as "publisher"
    Then the response status code should be 404
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" at "/mcp-servers" as "publisher" with definition "mcp" with values:
      | id   | ${CTX:mcpId} |
      | name | ${CTX:mcpId} |
      | type | MCP |
    Then the response status code should be 201

    # Creating an MCP server with no definition is refused and stores nothing
    Given I generate a unique resource name from "portal-invalid-mcp" and store it as "mcpId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" at "/mcp-servers" as "publisher" with definition "none" with values:
      | id   | ${CTX:mcpId} |
      | name | ${CTX:mcpId} |
      | type | MCP       |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:mcpId}" as "publisher"
    Then the response status code should be 404
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" at "/mcp-servers" as "publisher" with definition "mcp" with values:
      | id   | ${CTX:mcpId} |
      | name | ${CTX:mcpId} |
      | type | MCP |
    Then the response status code should be 201

    # Updating an MCP server with no type is refused and it stays an MCP server
    Given I generate a unique resource name from "portal-retyped-mcp" and store it as "mcpId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" at "/mcp-servers" as "publisher" with definition "mcp" with values:
      | id   | ${CTX:mcpId} |
      | name | ${CTX:mcpId} |
      | type | MCP |
    And the response status code should be 201
    When I update the API Portal API "${CTX:mcpId}" from "resources/templates/api-portal-api.yaml" at "/mcp-servers" as "publisher" with definition "mcp" with values:
      | id   | ${CTX:mcpId}         |
      | name | ${CTX:mcpId}-changed |
      | type | null               |
    Then the response status code should be 400
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:mcpId}" as "publisher"
    Then the JSON response field "type" should be "Mcp"
    And the JSON response field "name" should be "${CTX:mcpId}"

    # Updating an MCP server with the REST type is refused and it stays an MCP server
    Given I generate a unique resource name from "portal-retyped-mcp" and store it as "mcpId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" at "/mcp-servers" as "publisher" with definition "mcp" with values:
      | id   | ${CTX:mcpId} |
      | name | ${CTX:mcpId} |
      | type | MCP |
    And the response status code should be 201
    When I update the API Portal API "${CTX:mcpId}" from "resources/templates/api-portal-api.yaml" at "/mcp-servers" as "publisher" with definition "mcp" with values:
      | id   | ${CTX:mcpId}         |
      | name | ${CTX:mcpId}-changed |
      | type | REST               |
    Then the response status code should be 400
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:mcpId}" as "publisher"
    Then the JSON response field "type" should be "Mcp"
    And the JSON response field "name" should be "${CTX:mcpId}"

    # An MCP server sent to the REST API collection is refused
    Given I generate a unique resource name from "portal-mcp-as-api" and store it as "mcpId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "mcp" with values:
      | id   | ${CTX:mcpId} |
      | name | ${CTX:mcpId} |
      | type | MCP          |
    Then the response status code should be 400
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:mcpId}" as "publisher"
    Then the response status code should be 404

    # REST API and MCP server handles never resolve through each other's endpoints
    Given a REST API is created in the API Portal and stored as "apiId"
    And I generate a unique resource name from "portal-isolated-mcp" and store it as "mcpId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" at "/mcp-servers" as "publisher" with definition "mcp" with values:
      | id   | ${CTX:mcpId} |
      | name | ${CTX:mcpId} |
      | type | MCP |
    And the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:mcpId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:apiId}" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:mcpId}" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/mcp-servers?name=${CTX:apiId}" as "publisher"
    Then the response body should not contain "${CTX:apiId}"
    When I send an authenticated API Portal "GET" request to "/apis?name=${CTX:mcpId}" as "publisher"
    Then the response body should not contain "${CTX:mcpId}"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/api-keys" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:mcpId}/api-keys" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:mcpId}/api-keys" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:apiId}/api-keys" as "publisher"
    Then the response status code should be 404

    # MCP server requests that repeat an id, name nothing or are not allowed are refused
    Given I generate a unique resource name from "portal-guarded-mcp" and store it as "mcpId"
    And I generate a unique resource name from "portal-missing-mcp" and store it as "missingMcpId"
    And I generate a unique resource name from "portal-developer-mcp" and store it as "developerMcpId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" at "/mcp-servers" as "publisher" with definition "mcp" with values:
      | id   | ${CTX:mcpId} |
      | name | ${CTX:mcpId} |
      | type | MCP |
    And the response status code should be 201
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" at "/mcp-servers" as "publisher" with definition "mcp" with values:
      | id   | ${CTX:mcpId} |
      | name | ${CTX:mcpId}-second |
      | type | MCP |
    Then the response status code should be 409
    And the response body should not contain "constraint"
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:mcpId}" as "publisher"
    Then the JSON response field "name" should be "${CTX:mcpId}"
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:missingMcpId}" as "publisher"
    Then the response status code should be 404
    When I update the API Portal API "${CTX:missingMcpId}" from "resources/templates/api-portal-api.yaml" at "/mcp-servers" as "publisher" with definition "mcp" with values:
      | id   | ${CTX:missingMcpId} |
      | name | ${CTX:missingMcpId} |
      | type | MCP                 |
    Then the response status code should be 404
    When I send an authenticated API Portal "DELETE" request to "/mcp-servers/${CTX:missingMcpId}" as "publisher"
    Then the response status code should be 404
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" at "/mcp-servers" as "developer" with definition "mcp" with values:
      | id   | ${CTX:developerMcpId} |
      | name | ${CTX:developerMcpId} |
      | type | MCP                   |
    Then the response status code should be 403
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:developerMcpId}" as "publisher"
    Then the response status code should be 404
    When I send an unauthenticated API Portal "GET" request to "/mcp-servers"
    Then the response status code should be 401
    When I send an authenticated API Portal "GET" request to "/mcp-servers" as "developer"
    Then the response status code should be 200

    # An MCP server with a subscription cannot be deleted until the subscription is removed
    Given I generate a unique resource name from "portal-subscribed-mcp" and store it as "mcpId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" at "/mcp-servers" as "publisher" with definition "mcp" with values:
      | id                | ${CTX:mcpId}       |
      | name              | ${CTX:mcpId}       |
      | type              | MCP                |
      | subscriptionPlans | [{"id": "Gold"}]   |
    And the response status code should be 201
    When a developer subscription for API "${CTX:mcpId}" using plan "Gold" is created and stored as "subscriptionId"
    Then the response status code should be 201
    And the JSON response should have field "subscriptionToken"
    And the JSON response field "subscriptionPlanName" should be "Gold"
    When I send an authenticated API Portal "DELETE" request to "/mcp-servers/${CTX:mcpId}" as "publisher"
    Then the response status code should be 409
    And the response body should contain "API has subscriptions."
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:mcpId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "DELETE" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the response status code should be 200
    When I send an authenticated API Portal "DELETE" request to "/mcp-servers/${CTX:mcpId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:mcpId}" as "publisher" until status 404
    Then the response status code should be 404
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers/${CTX:mcpId}/versions/v1.0"
    Then the response status code should be 404

  @APIP-018
  Scenario: An MCP server key is listed, regenerated, linked to an application and revoked for good
    # An MCP server key is listed, regenerated, linked to an application and revoked for good
    # Keys and applications belong to the user who created them, so one administrator owns both here;
    # another user's application is not found.
    Given I generate a unique resource name from "portal-keyed-mcp" and store it as "mcpId"
    And I generate a unique resource name from "portal-mcp-key" and store it as "keyId"
    And I generate a unique resource name from "portal-mcp-key-app" and store it as "appId"
    And I generate a unique resource name from "portal-mcp-foreign-app" and store it as "foreignAppId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" at "/mcp-servers" as "publisher" with definition "mcp" with values:
      | id   | ${CTX:mcpId} |
      | name | ${CTX:mcpId} |
      | type | MCP          |
    And the response status code should be 201
    And a unique API Portal resource is created at "/applications" as "admin" with values and stored as "appId":
      | id          | ${CTX:appId}         |
      | displayName | MCP Key Application  |
      | description | Holds an MCP key     |
    And a unique API Portal resource is created at "/applications" as "developer" with values and stored as "foreignAppId":
      | id          | ${CTX:foreignAppId}    |
      | displayName | Another User's App     |
      | description | Owned by the developer |
    When I send an authenticated API Portal "POST" request to "/mcp-servers/${CTX:mcpId}/api-keys/generate" as "admin" with JSON values:
      | id          | ${CTX:keyId}  |
      | displayName | Agent MCP Key |
    Then the response status code should be 201
    And the JSON response field "status" should be "ACTIVE"
    And I store the JSON response field "key" as "originalKey"
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:mcpId}/api-keys" as "admin"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:keyId}"
    And the JSON response array "list" item with "id" equal to "${CTX:keyId}" should have "status" equal to "ACTIVE"
    And the JSON response array "list" item with "id" equal to "${CTX:keyId}" should not have field "key"
    When I send an authenticated API Portal "POST" request to "/mcp-servers/${CTX:mcpId}/api-keys/regenerate" as "admin" with JSON values:
      | keyId | ${CTX:keyId} |
    Then the response status code should be 200
    And the JSON response field "key" should not equal "${CTX:originalKey}"
    When I send an authenticated API Portal "POST" request to "/mcp-servers/${CTX:mcpId}/api-keys/associate" as "admin" with JSON values:
      | keyId | ${CTX:keyId}        |
      | appId | ${CTX:foreignAppId} |
    Then the response status code should be 404
    And the response body should not contain "stack"
    When I send an authenticated API Portal "POST" request to "/mcp-servers/${CTX:mcpId}/api-keys/associate" as "admin" with JSON values:
      | keyId | ${CTX:keyId} |
      | appId | ${CTX:appId} |
    Then the response status code should be 200
    And the JSON response field "application.id" should be "${CTX:appId}"
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:mcpId}/api-keys?appId=${CTX:appId}" as "admin"
    Then the JSON response array "list" should contain an item with "id" equal to "${CTX:keyId}"
    When I send an authenticated API Portal "POST" request to "/mcp-servers/${CTX:mcpId}/api-keys/dissociate" as "admin" with JSON values:
      | keyId | ${CTX:keyId} |
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:mcpId}/api-keys?appId=${CTX:appId}" as "admin"
    Then the JSON response array "list" should not contain an item with "id" equal to "${CTX:keyId}"
    When I send an authenticated API Portal "POST" request to "/mcp-servers/${CTX:mcpId}/api-keys/revoke" as "admin" with JSON values:
      | keyId | ${CTX:keyId} |
    Then the response status code should be 204
    When I send an authenticated API Portal "POST" request to "/mcp-servers/${CTX:mcpId}/api-keys/revoke" as "admin" with JSON values:
      | keyId | ${CTX:keyId} |
    Then the response status code should be 409
    When I send an authenticated API Portal "POST" request to "/mcp-servers/${CTX:mcpId}/api-keys/regenerate" as "admin" with JSON values:
      | keyId | ${CTX:keyId} |
    Then the response status code should be 409
    When I send an authenticated API Portal "POST" request to "/mcp-servers/${CTX:mcpId}/api-keys/associate" as "admin" with JSON values:
      | keyId | ${CTX:keyId} |
      | appId | ${CTX:appId} |
    Then the response status code should be 409
    When I send an authenticated API Portal "DELETE" request to "/mcp-servers/${CTX:mcpId}" as "publisher"
    Then the response status code should be 200

  @APIP-019 @type:negative
  Scenario: MCP server versions are published through the registry, change status and are soft-deleted
    # A publisher publishes an MCP server version through the registry, which lists it under both paths
    Given I generate a unique resource name from "portal-registry" and store it as "serverName"
    When I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON values:
      | name        | example.com/${CTX:serverName} |
      | description | Plans trips                   |
      | version     | 1.0.0                         |
    Then the response status code should be 201
    And I register API Portal resource "${CTX:serverName}" at "/mcp-servers" for cleanup
    When I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON values:
      | name        | example.com/${CTX:serverName} |
      | description | Plans trips anywhere          |
      | version     | 1.0.0                         |
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers?search=${CTX:serverName}"
    Then the response status code should be 200
    And the response body should contain "example.com/${CTX:serverName}"
    And the response header "Access-Control-Allow-Origin" should be "*"
    And the response header "Access-Control-Allow-Methods" should contain "GET"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/registry/v0.1/servers?search=${CTX:serverName}"
    Then the response status code should be 200
    And the response body should contain "example.com/${CTX:serverName}"
    And the response header "Access-Control-Allow-Origin" should be "*"
    And the response header "Access-Control-Allow-Methods" should contain "GET"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions"
    Then the response status code should be 200
    And the response body should contain "1.0.0"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.0.0"
    Then the response status code should be 200
    And the response body should contain "Plans trips anywhere"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/9.9.9"
    Then the response status code should be 404

    # Capabilities published to the registry become the MCP server's tools, and an update without them keeps them
    Given I generate a unique resource name from "portal-registry-tools" and store it as "serverName"
    When I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON values:
      | name        | example.com/${CTX:serverName} |
      | description | Plans trips                   |
      | version     | 1.0.0                         |
      | _meta       | {"io.api-platform/mcp-capabilities":{"tools":[{"name":"plan_trip","description":"Plans a trip","inputSchema":{"type":"object","properties":{}}}]},"io.api-platform/proxy-info":{"id":"${CTX:serverName}"}} |
    Then the response status code should be 201
    And I register API Portal resource "${CTX:serverName}" at "/mcp-servers" for cleanup
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.0.0"
    Then the response status code should be 200
    And the response body should contain "plan_trip"
    And the response body should contain "TOOL"
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:serverName}/assets?type=SCHEMA_DEFINITION&fileName=definition.yaml" as "publisher"
    Then the response status code should be 200
    And the response body should contain "name: plan_trip"
    And the response body should contain "type: TOOL"
    When I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON values:
      | name        | example.com/${CTX:serverName} |
      | description | Plans trips anywhere          |
      | version     | 1.0.0                         |
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.0.0"
    Then the response status code should be 200
    And the response body should contain "Plans trips anywhere"
    And the response body should contain "plan_trip"
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:serverName}/assets?type=SCHEMA_DEFINITION&fileName=definition.yaml" as "publisher"
    Then the response body should contain "name: plan_trip"
    When I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON values:
      | name        | example.com/${CTX:serverName} |
      | description | Books hotels                  |
      | version     | 1.0.0                         |
      | _meta       | {"io.api-platform/mcp-capabilities":{"tools":[{"name":"book_hotel","description":"Books a hotel","inputSchema":{"type":"object","properties":{}}}]}} |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/mcp-servers/${CTX:serverName}/assets?type=SCHEMA_DEFINITION&fileName=definition.yaml" as "publisher"
    Then the response body should contain "name: book_hotel"
    And the response body should not contain "plan_trip"

    # Registry changes reach only a version that exists, and an update with capabilities replaces the tools
    Given I generate a unique resource name from "portal-registry-update" and store it as "serverName"
    And I generate a unique resource name from "portal-registry-missing" and store it as "missingName"
    And I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON values:
      | name        | example.com/${CTX:serverName} |
      | description | Plans trips                   |
      | version     | 1.0.0                         |
      | _meta       | {"io.api-platform/mcp-capabilities":{"tools":[{"name":"plan_trip","description":"Plans a trip","inputSchema":{"type":"object","properties":{}}}]}} |
    And the response status code should be 201
    And I register API Portal resource "${CTX:serverName}" at "/mcp-servers" for cleanup
    When I send an authenticated API Portal "PUT" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/9.9.9" as "publisher" with JSON values:
      | name        | example.com/${CTX:serverName} |
      | description | Never stored                  |
      | version     | 9.9.9                         |
    Then the response status code should be 404
    When I send an authenticated API Portal "PATCH" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/9.9.9/status" as "publisher" with JSON values:
      | status | deprecated |
    Then the response status code should be 404
    When I send an authenticated API Portal "DELETE" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/9.9.9" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "PATCH" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:missingName}/status" as "publisher" with JSON values:
      | status | deprecated |
    Then the response status code should be 404
    When I send an authenticated API Portal "PUT" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.0.0" as "publisher" with JSON values:
      | name    | example.com/${CTX:serverName} |
      | version | 1.0.0                         |
    Then the response status code should be 400
    And the JSON response should have field "error"
    When I send an authenticated API Portal "PUT" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.0.0" as "publisher" with JSON values:
      | name        | example.com/${CTX:serverName} |
      | description | Plans trips and hotels        |
      | version     | 1.0.0                         |
      | _meta       | {"io.api-platform/mcp-capabilities":{"tools":[{"name":"book_hotel","description":"Books a hotel","inputSchema":{"type":"object","properties":{}}}]}} |
    Then the response status code should be 200
    And the response body should contain "book_hotel"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.0.0"
    Then the response body should contain "Plans trips and hotels"
    And the response body should contain "book_hotel"
    And the response body should not contain "plan_trip"
    When I send an authenticated API Portal "PUT" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.0.0" as "publisher" with JSON values:
      | name        | example.com/${CTX:serverName} |
      | description | Plans trips again             |
      | version     | 1.0.0                         |
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.0.0"
    Then the response body should contain "Plans trips again"
    And the response body should contain "book_hotel"

    # A registry version's status changes only to a new allowed value, and deleting it is soft
    Given I generate a unique resource name from "portal-registry-status" and store it as "serverName"
    And I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON values:
      | name        | example.com/${CTX:serverName} |
      | description | Plans trips                   |
      | version     | 1.0.0                         |
    And the response status code should be 201
    And I register API Portal resource "${CTX:serverName}" at "/mcp-servers" for cleanup
    When I send an authenticated API Portal "PUT" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.0.0" as "publisher" with JSON values:
      | name        | example.com/${CTX:serverName} |
      | description | Plans trips                   |
      | version     | 2.0.0                         |
    Then the response status code should be 400
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.0.0"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/2.0.0"
    Then the response status code should be 404
    When I send an authenticated API Portal "PATCH" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.0.0/status" as "publisher" with JSON values:
      | status | deprecated |
    Then the response status code should be 200
    When I send an authenticated API Portal "PATCH" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.0.0/status" as "publisher" with JSON values:
      | status | deprecated |
    Then the response status code should be 400
    When I send an authenticated API Portal "PATCH" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.0.0/status" as "publisher" with JSON values:
      | status | retired |
    Then the response status code should be 400
    When I send an authenticated API Portal "DELETE" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.0.0" as "publisher"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.0.0"
    Then the response status code should be 404
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.0.0?include_deleted=true"
    Then the response status code should be 200

    # A server-level status change applies to the registry server's version and can be undone
    Given I generate a unique resource name from "portal-registry-server-status" and store it as "serverName"
    And I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON values:
      | name        | example.com/${CTX:serverName} |
      | description | Plans trips                   |
      | version     | 1.0.0                         |
    And the response status code should be 201
    And I register API Portal resource "${CTX:serverName}" at "/mcp-servers" for cleanup
    When I send an authenticated API Portal "PATCH" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/status" as "publisher" with JSON values:
      | status | deprecated |
    Then the response status code should be 200
    And the JSON response field "updatedCount" should be 1
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.0.0"
    Then the response body should contain "deprecated"
    And the response body should not contain "active"
    When I send an authenticated API Portal "PATCH" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/status" as "publisher" with JSON values:
      | status | unknown |
    Then the response status code should be 400
    When I send an authenticated API Portal "PATCH" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/status" as "publisher" with JSON values:
      | status | active |
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.0.0"
    Then the response body should contain "active"
    And the response body should not contain "deprecated"

  @APIP-019 @type:boundary @type:negative
  Scenario: Invalid registry publishes are refused and nothing is listed, within the allowed name lengths
    # A registry server whose name has no namespace is refused and not listed
    Given I generate a unique resource name from "portal-registry-invalid" and store it as "serverName"
    When I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON values:
      | name        | ${CTX:serverName}        |
      | description | Plans trips   |
      | version     | 1.0.0     |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers?search=${CTX:serverName}"
    Then the response status code should be 200
    And the response body should not contain "${CTX:serverName}"

    # A registry server whose version is latest is refused and not listed
    Given I generate a unique resource name from "portal-registry-invalid" and store it as "serverName"
    When I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON values:
      | name        | example.com/${CTX:serverName}        |
      | description | Plans trips   |
      | version     | latest     |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers?search=${CTX:serverName}"
    Then the response status code should be 200
    And the response body should not contain "${CTX:serverName}"

    # A registry server whose version is a caret range is refused and not listed
    Given I generate a unique resource name from "portal-registry-invalid" and store it as "serverName"
    When I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON values:
      | name        | example.com/${CTX:serverName}        |
      | description | Plans trips   |
      | version     | ^1.0.0     |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers?search=${CTX:serverName}"
    Then the response status code should be 200
    And the response body should not contain "${CTX:serverName}"

    # A registry server whose version is a wildcard is refused and not listed
    Given I generate a unique resource name from "portal-registry-invalid" and store it as "serverName"
    When I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON values:
      | name        | example.com/${CTX:serverName}        |
      | description | Plans trips   |
      | version     | 1.x     |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers?search=${CTX:serverName}"
    Then the response status code should be 200
    And the response body should not contain "${CTX:serverName}"

    # A registry publish with a body that is not an object is refused with an error and nothing is listed
    Given I generate a unique resource name from "portal-registry-refused" and store it as "serverName"
    When I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON body:
      """
      ["example.com/${CTX:serverName}","1.0.0"]
      """
    Then the response status code should be 400
    And the JSON response should have field "error"
    And the response body should not contain "stack"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers?search=${CTX:serverName}"
    Then the response status code should be 200
    And the response body should not contain "${CTX:serverName}"

    # A registry publish with no name is refused with an error and nothing is listed
    Given I generate a unique resource name from "portal-registry-refused" and store it as "serverName"
    When I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON body:
      """
      {"description":"Plans trips","version":"1.0.0"}
      """
    Then the response status code should be 400
    And the JSON response should have field "error"
    And the response body should not contain "stack"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers?search=${CTX:serverName}"
    Then the response status code should be 200
    And the response body should not contain "${CTX:serverName}"

    # A registry publish with a name that is not a string is refused with an error and nothing is listed
    Given I generate a unique resource name from "portal-registry-refused" and store it as "serverName"
    When I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON body:
      """
      {"name":42,"description":"Plans trips","version":"1.0.0"}
      """
    Then the response status code should be 400
    And the JSON response should have field "error"
    And the response body should not contain "stack"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers?search=${CTX:serverName}"
    Then the response status code should be 200
    And the response body should not contain "${CTX:serverName}"

    # A registry publish with a name of 2 characters is refused with an error and nothing is listed
    Given I generate a unique resource name from "portal-registry-refused" and store it as "serverName"
    When I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON body:
      """
      {"name":"a/","description":"Plans trips","version":"1.0.0"}
      """
    Then the response status code should be 400
    And the JSON response should have field "error"
    And the response body should not contain "stack"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers?search=${CTX:serverName}"
    Then the response status code should be 200
    And the response body should not contain "${CTX:serverName}"

    # A registry publish with a name of 201 characters is refused with an error and nothing is listed
    Given I generate a unique resource name from "portal-registry-refused" and store it as "serverName"
    When I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON body:
      """
      {"name":"example.com/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","description":"Plans trips","version":"1.0.0"}
      """
    Then the response status code should be 400
    And the JSON response should have field "error"
    And the response body should not contain "stack"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers?search=${CTX:serverName}"
    Then the response status code should be 200
    And the response body should not contain "${CTX:serverName}"

    # A registry publish with no description is refused with an error and nothing is listed
    Given I generate a unique resource name from "portal-registry-refused" and store it as "serverName"
    When I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON body:
      """
      {"name":"example.com/${CTX:serverName}","version":"1.0.0"}
      """
    Then the response status code should be 400
    And the JSON response should have field "error"
    And the response body should not contain "stack"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers?search=${CTX:serverName}"
    Then the response status code should be 200
    And the response body should not contain "${CTX:serverName}"

    # A registry publish with a description that is a number is refused with an error and nothing is listed
    Given I generate a unique resource name from "portal-registry-refused" and store it as "serverName"
    When I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON body:
      """
      {"name":"example.com/${CTX:serverName}","description":7,"version":"1.0.0"}
      """
    Then the response status code should be 400
    And the JSON response should have field "error"
    And the response body should not contain "stack"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers?search=${CTX:serverName}"
    Then the response status code should be 200
    And the response body should not contain "${CTX:serverName}"

    # A registry publish with no version is refused with an error and nothing is listed
    Given I generate a unique resource name from "portal-registry-refused" and store it as "serverName"
    When I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON body:
      """
      {"name":"example.com/${CTX:serverName}","description":"Plans trips"}
      """
    Then the response status code should be 400
    And the JSON response should have field "error"
    And the response body should not contain "stack"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers?search=${CTX:serverName}"
    Then the response status code should be 200
    And the response body should not contain "${CTX:serverName}"

    # A registry publish with a tilde version range is refused with an error and nothing is listed
    Given I generate a unique resource name from "portal-registry-refused" and store it as "serverName"
    When I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON body:
      """
      {"name":"example.com/${CTX:serverName}","description":"Plans trips","version":"~1.0.0"}
      """
    Then the response status code should be 400
    And the JSON response should have field "error"
    And the response body should not contain "stack"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers?search=${CTX:serverName}"
    Then the response status code should be 200
    And the response body should not contain "${CTX:serverName}"

    # A registry publish with a minimum version range is refused with an error and nothing is listed
    Given I generate a unique resource name from "portal-registry-refused" and store it as "serverName"
    When I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON body:
      """
      {"name":"example.com/${CTX:serverName}","description":"Plans trips","version":">=1.0.0"}
      """
    Then the response status code should be 400
    And the JSON response should have field "error"
    And the response body should not contain "stack"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers?search=${CTX:serverName}"
    Then the response status code should be 200
    And the response body should not contain "${CTX:serverName}"

    # A registry publish with a star version is refused with an error and nothing is listed
    Given I generate a unique resource name from "portal-registry-refused" and store it as "serverName"
    When I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON body:
      """
      {"name":"example.com/${CTX:serverName}","description":"Plans trips","version":"*"}
      """
    Then the response status code should be 400
    And the JSON response should have field "error"
    And the response body should not contain "stack"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers?search=${CTX:serverName}"
    Then the response status code should be 200
    And the response body should not contain "${CTX:serverName}"

    # A registry server name of 200 characters, the longest allowed, is accepted
    Given I generate a unique resource name of 188 characters from "portal-registry-long" and store it as "longName"
    And I register API Portal resource "${CTX:longName}" at "/mcp-servers" for cleanup
    When I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON values:
      | name        | example.com/${CTX:longName} |
      | description | Plans trips |
      | version     | 1.0.0       |
    Then the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:longName}/versions/1.0.0"
    Then the response status code should be 200

  @APIP-020 @type:negative @type:security
  Scenario: The registry listing pages and searches under both paths, and only publishers can change it
    # The registry listing pages with a cursor, caps its limit and searches in any letter case
    Given I generate a unique resource name from "portal-registry-page" and store it as "prefix"
    And I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON values:
      | name        | example.com/${CTX:prefix}-Planner-1 |
      | description | Plans trips                         |
      | version     | 1.0.0                               |
    And the response status code should be 201
    And I register API Portal resource "${CTX:prefix}-Planner-1" at "/mcp-servers" for cleanup
    And I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON values:
      | name        | example.com/${CTX:prefix}-Planner-2 |
      | description | Plans trips                         |
      | version     | 1.0.0                               |
    And the response status code should be 201
    And I register API Portal resource "${CTX:prefix}-Planner-2" at "/mcp-servers" for cleanup
    And I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON values:
      | name        | example.com/${CTX:prefix}-Planner-3 |
      | description | Plans trips                         |
      | version     | 1.0.0                               |
    And the response status code should be 201
    And I register API Portal resource "${CTX:prefix}-Planner-3" at "/mcp-servers" for cleanup
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers?search=${CTX:prefix}-PLANNER&limit=2"
    Then the response status code should be 200
    And the JSON response array field "servers" should have 2 items
    And the JSON response should have field "metadata.nextCursor"
    And I store the JSON response field "metadata.nextCursor" as "nextCursor"
    And I store the API Portal response body as "firstPage"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers?search=${CTX:prefix}-PLANNER&limit=2&cursor=${CTX:nextCursor}"
    Then the response status code should be 200
    And the JSON response array field "servers" should have 1 item
    And the JSON response field "metadata.nextCursor" should not exist
    And I store the API Portal response body as "secondPage"
    And the values "${CTX:prefix}-Planner-1,${CTX:prefix}-Planner-2,${CTX:prefix}-Planner-3" should each appear in exactly one of the stored API Portal responses "firstPage,secondPage"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers?search=${CTX:prefix}-planner"
    Then the JSON response array field "servers" should have 3 items
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers?search=${CTX:prefix}&limit=100"
    Then the response status code should be 200
    And the JSON response array field "servers" should have 3 items
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers?search=${CTX:prefix}&limit=101"
    Then the response status code should be 200
    And the JSON response array field "servers" should have 3 items

    # The registry refuses a cursor it did not issue and an organization it does not serve
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers?limit=5"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers?cursor=not-a-cursor"
    Then the response status code should be 400
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/foreign-org/v0.1/servers"
    Then the response status code should be 404

    # A developer cannot publish to the MCP registry
    Given I generate a unique resource name from "portal-registry-denied" and store it as "serverName"
    And I register API Portal resource "${CTX:serverName}" at "/mcp-servers" for cleanup
    When I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "developer" with JSON values:
      | name        | example.com/${CTX:serverName} |
      | description | Not allowed                   |
      | version     | 1.0.0                         |
    Then the response status code should be 403
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions"
    Then the response status code should be 404

    # A developer cannot change, delete or set the status of a registry server, while anyone can still read it
    Given I generate a unique resource name from "portal-registry-guarded" and store it as "serverName"
    And I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON values:
      | name        | example.com/${CTX:serverName} |
      | description | Plans trips                   |
      | version     | 1.0.0                         |
    And the response status code should be 201
    And I register API Portal resource "${CTX:serverName}" at "/mcp-servers" for cleanup
    When I send an authenticated API Portal "PUT" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.0.0" as "developer" with JSON values:
      | name        | example.com/${CTX:serverName} |
      | description | Changed by a developer        |
      | version     | 1.0.0                         |
    Then the response status code should be 403
    When I send an authenticated API Portal "PATCH" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.0.0/status" as "developer" with JSON values:
      | status | deprecated |
    Then the response status code should be 403
    When I send an authenticated API Portal "PATCH" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/status" as "developer" with JSON values:
      | status | deprecated |
    Then the response status code should be 403
    When I send an authenticated API Portal "DELETE" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.0.0" as "developer"
    Then the response status code should be 403
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.0.0"
    Then the response status code should be 200
    And the response body should contain "Plans trips"
    And the response body should not contain "Changed by a developer"
    And the response body should not contain "deprecated"
    When I send an authenticated API Portal "PUT" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.0.0" as "publisher" with JSON values:
      | name        | example.com/${CTX:serverName} |
      | description | Changed by the publisher      |
      | version     | 1.0.0                         |
    Then the response status code should be 200

  @APIP-020 @known-issue
  Scenario: A second version of a registry server is published, listed beside the first and follows server-level status changes
    # A second version of a registry server is published, listed beside the first and follows server-level status changes
    # Known issue: the portal handle is derived from the server name and must be unique, so publishing
    # a second version of the same server fails with 409 "Server version already exists".
    Given I generate a unique resource name from "portal-registry-versions" and store it as "serverName"
    And I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON values:
      | name        | example.com/${CTX:serverName} |
      | description | Plans trips                   |
      | version     | 1.0.0                         |
    And the response status code should be 201
    And I register API Portal resource "${CTX:serverName}" at "/mcp-servers" for cleanup
    When I send an authenticated API Portal "POST" request to "/api-portal/registry/default/v0.1/publish" as "publisher" with JSON values:
      | name        | example.com/${CTX:serverName} |
      | description | Plans trips faster            |
      | version     | 1.1.0                         |
    Then the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions"
    Then the response status code should be 200
    And the JSON response array field "servers" should have 2 items
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.0.0"
    Then the response body should contain "Plans trips"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/versions/1.1.0"
    Then the response body should contain "Plans trips faster"
    When I send an authenticated API Portal "PATCH" request to "/api-portal/registry/default/v0.1/servers/example.com%2F${CTX:serverName}/status" as "publisher" with JSON values:
      | status | deprecated |
    Then the response status code should be 200
    And the JSON response field "updatedCount" should be 2
