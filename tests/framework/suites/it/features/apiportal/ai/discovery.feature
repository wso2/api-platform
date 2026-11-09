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


@APIPORTAL @apiportal-ai-discovery
Feature: AI agents discover a view's APIs, MCP servers and specifications without signing in
  As an API publisher
  I want each view to publish an llms.txt index, Markdown catalogues and raw specifications
  So that AI agents can find and call the APIs I make visible to them, and never the ones I hide

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml).
  # Every scenario after the first uses its own labelled view, so what it publishes, and turning AI
  # discovery off, never affects another runner's view. Every hidden entry is paired with a visible
  # one in the same index.

  @APIP-022
  Scenario: AI agents discover the catalogue through llms.txt, Markdown, specifications and robots.txt
    # The portal-wide llms.txt is the public entry point for the configured organization
    When I send an unauthenticated API Portal public "GET" request to "/llms.txt"
    Then the response status code should be 200
    And the response body should contain "AI Agent Entry Point"
    And the response body should contain "/default/views/"
    And the response body should not contain "{orgName}"

    # A view's llms.txt has a section only for the kinds of API it shows
    Given I generate a unique resource name from "portal-discovery-label" and store it as "labelId"
    And I generate a unique resource name from "portal-discovery-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId}  |
      | displayName | Discovery Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Discovery View     |
      | labels      | ["${CTX:labelId}"] |
    And I generate a unique resource name from "portal-discovery-rest" and store it as "restApiId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "rest" with values:
      | id     | ${CTX:restApiId}       |
      | name   | ${CTX:restApiId}       |
      | type   | REST             |
      | labels | ["${CTX:labelId}"] |
    And the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/llms.txt"
    Then the response status code should be 200
    And the response body should contain "## APIs"
    And the response body should not contain "## GraphQL APIs"
    When I generate a unique resource name from "portal-discovery-graphql" and store it as "graphqlApiId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "graphql" with values:
      | id     | ${CTX:graphqlApiId}       |
      | name   | ${CTX:graphqlApiId}       |
      | type   | GRAPHQL             |
      | labels | ["${CTX:labelId}"] |
    And the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/llms.txt"
    Then the response body should contain "## GraphQL APIs"
    And the response body should contain "${CTX:graphqlApiId}"

    # A view lists its visible APIs and MCP servers to agents, and leaves out hidden ones
    Given I generate a unique resource name from "portal-discovery-label" and store it as "labelId"
    And I generate a unique resource name from "portal-discovery-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId}  |
      | displayName | Discovery Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Discovery View     |
      | labels      | ["${CTX:labelId}"] |
    And I generate a unique resource name from "portal-discovery-rest" and store it as "restApiId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "rest" with values:
      | id     | ${CTX:restApiId}       |
      | name   | ${CTX:restApiId}       |
      | type   | REST             |
      | labels | ["${CTX:labelId}"] |
    And the response status code should be 201
    And I generate a unique resource name from "portal-discovery-graphql" and store it as "graphqlApiId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "graphql" with values:
      | id     | ${CTX:graphqlApiId}       |
      | name   | ${CTX:graphqlApiId}       |
      | type   | GRAPHQL             |
      | labels | ["${CTX:labelId}"] |
    And the response status code should be 201
    And I generate a unique resource name from "portal-discovery-websocket" and store it as "websocketApiId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "websocket" with values:
      | id     | ${CTX:websocketApiId}       |
      | name   | ${CTX:websocketApiId}       |
      | type   | WS             |
      | labels | ["${CTX:labelId}"] |
    And the response status code should be 201
    And I generate a unique resource name from "portal-discovery-websub" and store it as "websubApiId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "websub" with values:
      | id     | ${CTX:websubApiId}       |
      | name   | ${CTX:websubApiId}       |
      | type   | WEBSUB             |
      | labels | ["${CTX:labelId}"] |
    And the response status code should be 201
    And I generate a unique resource name from "portal-discovery-rest" and store it as "hiddenApiId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "rest" with values:
      | id     | ${CTX:hiddenApiId}       |
      | name   | ${CTX:hiddenApiId}       |
      | type   | REST             |
      | labels | ["${CTX:labelId}"] |
      | agentVisibility | HIDDEN |
    And the response status code should be 201
    And a unique API Portal MCP server with label "${CTX:labelId}" is created and stored as "mcpId"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/llms.txt"
    Then the response status code should be 200
    And the response body should contain "${CTX:restApiId}"
    And the response body should contain "${CTX:graphqlApiId}"
    And the response body should contain "${CTX:websocketApiId}"
    And the response body should contain "${CTX:websubApiId}"
    And the response body should not contain "${CTX:hiddenApiId}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/apis.md"
    Then the response status code should be 200
    And the response body should contain "${CTX:restApiId}"
    And the response body should contain "${CTX:websubApiId}"
    And the response body should not contain "${CTX:hiddenApiId}"
    And the response body should not contain "${CTX:mcpId}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/mcps.md"
    Then the response status code should be 200
    And the response body should contain "${CTX:mcpId}"
    And the response body should not contain "${CTX:restApiId}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:hiddenApiId}.md"
    Then the response status code should be 404
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:restApiId}.md"
    Then the response status code should be 200

    # A REST API's specification is served raw in its own format only
    Given I generate a unique resource name from "portal-discovery-label" and store it as "labelId"
    And I generate a unique resource name from "portal-discovery-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId}  |
      | displayName | Discovery Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Discovery View     |
      | labels      | ["${CTX:labelId}"] |
    And I generate a unique resource name from "portal-spec-api" and store it as "apiId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "rest" with values:
      | id     | ${CTX:apiId}       |
      | name   | ${CTX:apiId}       |
      | type   | REST             |
      | labels | ["${CTX:labelId}"] |
    And the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}/docs/specification.json"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}/docs/specification.graphql"
    Then the response status code should be 404

    # A WebSocket API's specification is served raw in its own format only
    Given I generate a unique resource name from "portal-discovery-label" and store it as "labelId"
    And I generate a unique resource name from "portal-discovery-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId}  |
      | displayName | Discovery Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Discovery View     |
      | labels      | ["${CTX:labelId}"] |
    And I generate a unique resource name from "portal-spec-api" and store it as "apiId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "websocket" with values:
      | id     | ${CTX:apiId}       |
      | name   | ${CTX:apiId}       |
      | type   | WS             |
      | labels | ["${CTX:labelId}"] |
    And the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}/docs/specification.json"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}/docs/specification.xml"
    Then the response status code should be 404

    # A WebSub API's specification is served raw in its own format only
    Given I generate a unique resource name from "portal-discovery-label" and store it as "labelId"
    And I generate a unique resource name from "portal-discovery-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId}  |
      | displayName | Discovery Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Discovery View     |
      | labels      | ["${CTX:labelId}"] |
    And I generate a unique resource name from "portal-spec-api" and store it as "apiId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "websub" with values:
      | id     | ${CTX:apiId}       |
      | name   | ${CTX:apiId}       |
      | type   | WEBSUB             |
      | labels | ["${CTX:labelId}"] |
    And the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}/docs/specification.json"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}/docs/specification.graphql"
    Then the response status code should be 404

    # A GraphQL API's specification is served raw in its own format only
    Given I generate a unique resource name from "portal-discovery-label" and store it as "labelId"
    And I generate a unique resource name from "portal-discovery-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId}  |
      | displayName | Discovery Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Discovery View     |
      | labels      | ["${CTX:labelId}"] |
    And I generate a unique resource name from "portal-spec-api" and store it as "apiId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "graphql" with values:
      | id     | ${CTX:apiId}       |
      | name   | ${CTX:apiId}       |
      | type   | GRAPHQL             |
      | labels | ["${CTX:labelId}"] |
    And the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}/docs/specification.graphql"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}/docs/specification.json"
    Then the response status code should be 404

    # A SOAP API's specification is served raw in its own format only
    Given I generate a unique resource name from "portal-discovery-label" and store it as "labelId"
    And I generate a unique resource name from "portal-discovery-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId}  |
      | displayName | Discovery Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Discovery View     |
      | labels      | ["${CTX:labelId}"] |
    And I generate a unique resource name from "portal-spec-api" and store it as "apiId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "soap" with values:
      | id     | ${CTX:apiId}       |
      | name   | ${CTX:apiId}       |
      | type   | SOAP             |
      | labels | ["${CTX:labelId}"] |
    And the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}/docs/specification.xml"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}/docs/specification.json"
    Then the response status code should be 404

    # An MCP server's tools are served as its specification
    Given I generate a unique resource name from "portal-discovery-label" and store it as "labelId"
    And I generate a unique resource name from "portal-discovery-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId}  |
      | displayName | Discovery Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Discovery View     |
      | labels      | ["${CTX:labelId}"] |
    And a unique API Portal MCP server with label "${CTX:labelId}" is created and stored as "mcpId"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/mcp/${CTX:mcpId}/docs/specification.json"
    Then the response status code should be 200
    And the response body should contain "ping"

    # The API page that Try with AI points to names the API, its specification and how to get credentials
    Given I generate a unique resource name from "portal-discovery-label" and store it as "labelId"
    And I generate a unique resource name from "portal-discovery-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId}  |
      | displayName | Discovery Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Discovery View     |
      | labels      | ["${CTX:labelId}"] |
    And I generate a unique resource name from "portal-prompt-api" and store it as "apiId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "rest" with values:
      | id     | ${CTX:apiId}       |
      | name   | ${CTX:apiId}       |
      | type   | REST               |
      | labels | ["${CTX:labelId}"] |
    And the response status code should be 201
    And I generate a unique resource name from "portal-prompt-hidden-api" and store it as "hiddenApiId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "rest" with values:
      | id              | ${CTX:hiddenApiId} |
      | name            | ${CTX:hiddenApiId} |
      | type            | REST               |
      | labels          | ["${CTX:labelId}"] |
      | agentVisibility | HIDDEN             |
    And the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}.md"
    Then the response status code should be 200
    And the response body should contain "# ${CTX:apiId}"
    And the response body should contain "## Authentication"
    And the response body should contain "/api/${CTX:apiId}/docs/specification.json"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}/docs/specification.json"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:hiddenApiId}.md"
    Then the response status code should be 404

    # robots.txt lets crawlers in and points AI agents to a view's llms.txt
    When I send an unauthenticated API Portal "GET" request to the root path "/robots.txt"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "text/plain"
    And the response body should contain "User-agent: *"
    And the response body should contain "Allow: /"
    And the response body should contain "/api-portal/default/views/"
    And the response body should contain "llms.txt"

    # A view with AI discovery turned off serves no llms.txt
    Given I generate a unique resource name from "portal-discovery-label" and store it as "labelId"
    And I generate a unique resource name from "portal-discovery-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId}  |
      | displayName | Discovery Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Discovery View     |
      | labels      | ["${CTX:labelId}"] |
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/llms.txt"
    Then the response status code should be 200
    When I verify API Portal view "${CTX:viewId}" returns not found when AI discovery is disabled as "publisher"
    Then the response status code should be 404
