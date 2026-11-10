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

@platform-api-builds
Feature: Builds are immutable snapshots each artifact kind prepares, lists and deletes
  As an API publisher
  I want to prepare a build of an artifact, find it again and remove it when it is no longer
  needed, the same way for every artifact kind
  So that what I deploy is a known snapshot rather than whatever the artifact is at that moment

  # One artifact of each kind that has its own build endpoints. The REST API and the MCP proxy
  # deliberately share a handle: handles are unique only within a kind.
  Background:
    Given I generate a unique resource name from "bld-proj" and store it as "project"
    And I generate a unique resource name from "bld-shared" and store it as "shared"
    And I generate a unique resource name from "bld-gql" and store it as "graphql"
    And I generate a unique resource name from "bld-prov" and store it as "provider"
    And I generate a unique resource name from "bld-proxy" and store it as "proxy"
    And I generate a unique API context from "/bld-rest" and store it as "restContext"
    And I generate a unique API context from "/bld-mcp" and store it as "mcpContext"
    And I generate a unique API context from "/bld-gql" and store it as "graphqlContext"
    And I generate a unique API context from "/bld-prov" and store it as "providerContext"
    And I generate a unique API context from "/bld-proxy" and store it as "proxyContext"
    And I create a project "${CTX:project}" on the control plane
    When I send a "POST" request to the control plane at "/rest-apis" with body:
      """
      {"id": "${CTX:shared}", "displayName": "${CTX:shared}", "context": "${CTX:restContext}", "version": "v1", "projectId": "${CTX:project}", "upstream": {"main": {"url": "http://backend.example.com"}}}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/mcp-proxies" with body:
      """
      {"id": "${CTX:shared}", "displayName": "${CTX:shared}", "version": "v1.0", "context": "${CTX:mcpContext}", "projectId": "${CTX:project}", "upstream": {"main": {"url": "http://mcp.example.com/mcp"}}}
      """
    Then the response status code should be 201
    And I create a GraphQL API "${CTX:graphql}" via the control plane in project "${CTX:project}" with context "${CTX:graphqlContext}"
    When I send a "POST" request to the control plane at "/llm-providers" with body:
      """
      {"id": "${CTX:provider}", "displayName": "${CTX:provider}", "version": "v1.0", "context": "${CTX:providerContext}", "template": "openai", "upstream": {"main": {"url": "http://llm.example.com"}}, "accessControl": {"mode": "allow_all"}}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/llm-proxies" with body:
      """
      {"id": "${CTX:proxy}", "displayName": "${CTX:proxy}", "version": "v1.0", "projectId": "${CTX:project}", "context": "${CTX:proxyContext}", "provider": {"id": "${CTX:provider}"}}
      """
    Then the response status code should be 201

  Scenario Outline: Every artifact kind prepares, lists, reads and deletes its own builds
    When I send a "POST" request to the control plane at "/<collection>/${CTX:<artifact>}/builds"
    Then the response status code should be 201
    And I store the JSON response field "buildId" as "firstBuild"
    And the response header "Location" should match pattern "^/api/v0\.9/<collection>/${CTX:<artifact>}/builds/[0-9]{4}-[0-9]{2}-[0-9]{2}-[0-9]+$"
    And the response header "Location" should be "/api/v0.9/<collection>/${CTX:<artifact>}/builds/${CTX:firstBuild}"
    And the JSON response should have field "uuid"
    And the JSON response should have field "dataVersion"
    And the JSON response field "createdBy" should be "admin"
    When I send a "POST" request to the control plane at "/<collection>/${CTX:<artifact>}/builds" with body:
      """
      {"description": "  Adds the reports endpoint  ", "metadata": {"ticket": "QA-1"}}
      """
    Then the response status code should be 201
    And I store the JSON response field "buildId" as "secondBuild"
    And the JSON response field "description" should be "Adds the reports endpoint"
    And the JSON response field "metadata.ticket" should be "QA-1"
    When I send a "GET" request to the control plane at "/<collection>/${CTX:<artifact>}/builds/${CTX:secondBuild}"
    Then the response status code should be 200
    And the JSON response field "description" should be "Adds the reports endpoint"
    And the JSON response field "createdBy" should be "admin"
    When I send a "GET" request to the control plane at "/<collection>/${CTX:<artifact>}/builds"
    Then the response status code should be 200
    And the JSON response field "count" should be 2
    And the JSON response field "list[0].buildId" should be "${CTX:secondBuild}"
    And the JSON response field "list[1].buildId" should be "${CTX:firstBuild}"
    And the JSON response field "list[0].content" should not exist
    And the JSON response field "list[0].createdBy" should be "admin"
    When I send a "GET" request to the control plane at "/<collection>/${CTX:<artifact>}/builds?limit=1"
    Then the JSON response array field "list" should have 1 item
    When I send a "DELETE" request to the control plane at "/<collection>/${CTX:<artifact>}/builds/${CTX:firstBuild}"
    Then the response status code should be 204
    When I send a "GET" request to the control plane at "/<collection>/${CTX:<artifact>}/builds/${CTX:firstBuild}"
    Then the response status code should be 404
    And the JSON response field "code" should be "BUILD_NOT_FOUND"
    When I send a "GET" request to the control plane at "/<collection>/${CTX:<artifact>}/builds"
    Then the JSON response field "count" should be 1

    Examples:
      | collection    | artifact |
      | rest-apis     | shared   |
      | mcp-proxies   | shared   |
      | graphql-apis  | graphql  |
      | llm-providers | provider |
      | llm-proxies   | proxy    |

  Scenario Outline: A kind's build endpoints do not reach an artifact of another kind
    When I send a "<method>" request to the control plane at "/<collection>/${CTX:graphql}/builds"
    Then the response status code should be 404
    And the JSON response field "code" should be "<code>"

    Examples:
      | method | collection    | code                   |
      | POST   | rest-apis     | REST_API_NOT_FOUND     |
      | GET    | mcp-proxies   | MCP_PROXY_NOT_FOUND    |
      | POST   | llm-providers | LLM_PROVIDER_NOT_FOUND |
      | GET    | llm-proxies   | LLM_PROXY_NOT_FOUND    |

  Scenario: A build is addressed only through the artifact it was prepared from
    Given I generate a unique resource name from "bld-other" and store it as "other"
    And I generate a unique API context from "/bld-other" and store it as "otherContext"
    When I send a "POST" request to the control plane at "/rest-apis" with body:
      """
      {"id": "${CTX:other}", "displayName": "${CTX:other}", "context": "${CTX:otherContext}", "version": "v1", "projectId": "${CTX:project}", "upstream": {"main": {"url": "http://backend.example.com"}}}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:shared}/builds"
    Then the response status code should be 201
    And I store the JSON response field "buildId" as "build"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:other}/builds/${CTX:build}"
    Then the response status code should be 404
    And the JSON response field "code" should be "BUILD_NOT_FOUND"
    When I send a "DELETE" request to the control plane at "/rest-apis/${CTX:other}/builds/${CTX:build}"
    Then the response status code should be 404
    When I send a "GET" request to the control plane at "/mcp-proxies/${CTX:shared}/builds/${CTX:build}"
    Then the response status code should be 404
    And the JSON response field "code" should be "BUILD_NOT_FOUND"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:shared}/builds/${CTX:build}"
    Then the response status code should be 200

  Scenario: A malformed build request is rejected
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:shared}/builds" with body:
      """
      {"description":
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"

  Scenario: Preparing past the limit prunes the oldest builds and never reuses an id
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:shared}/builds"
    Then the response status code should be 201
    And I store the JSON response field "buildId" as "oldest"
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:shared}/builds"
    Then the response status code should be 201
    And I store the JSON response field "buildId" as "secondOldest"
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:shared}/builds"
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:shared}/builds"
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:shared}/builds"
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:shared}/builds"
    Then the response status code should be 201
    And I store the JSON response field "buildId" as "newest"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:shared}/builds"
    Then the JSON response field "count" should be 5
    And the JSON response field "list[0].buildId" should be "${CTX:newest}"
    And the JSON response array "list" should not contain an item with "buildId" equal to "${CTX:oldest}"
    And the JSON response array "list" should contain an item with "buildId" equal to "${CTX:secondOldest}"

  Scenario Outline: Build scopes follow each role's grant
    When I send a "<method>" request to the control plane at "/<collection>/${CTX:<artifact>}/builds" as "<actor>"
    Then the response status code should be <status>

    Examples:
      | actor     | method | collection    | artifact | status |
      | developer | GET    | rest-apis     | shared   | 403    |
      | developer | POST   | rest-apis     | shared   | 403    |
      | publisher | POST   | llm-providers | provider | 403    |
      | publisher | POST   | rest-apis     | shared   | 201    |
      | publisher | GET    | mcp-proxies   | shared   | 200    |

  Scenario Outline: A deployment names the build it runs, and every base rule is enforced
    Given I generate a unique resource name from "bld-gw" and store it as "gateway"
    And I register a gateway "${CTX:gateway}" on the control plane
    When I send a "POST" request to the control plane at "/<collection>/${CTX:<artifact>}/deployments" with body:
      """
      {"name": "${UNIQUE:bld-dep}", "gatewayId": "${CTX:gateway}"}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "<code>"
    When I send a "POST" request to the control plane at "/<collection>/${CTX:<artifact>}/deployments" with body:
      """
      {"name": "${UNIQUE:bld-dep}", "base": "build", "gatewayId": "${CTX:gateway}"}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "<code>"
    When I send a "POST" request to the control plane at "/<collection>/${CTX:<artifact>}/deployments" with body:
      """
      {"name": "${UNIQUE:bld-dep}", "base": "current", "buildId": "2026-01-01-1", "gatewayId": "${CTX:gateway}"}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "<code>"
    When I send a "POST" request to the control plane at "/<collection>/${CTX:<artifact>}/deployments" with body:
      """
      {"name": "${UNIQUE:bld-dep}", "base": "build", "buildId": "2026-01-01-1", "gatewayId": "${CTX:gateway}"}
      """
    Then the response status code should be 404
    And the JSON response field "code" should be "BUILD_NOT_FOUND"
    When I send a "POST" request to the control plane at "/<collection>/${CTX:<artifact>}/deployments" with body:
      """
      {"name": "${UNIQUE:bld-dep}", "base": "current", "gatewayId": "${CTX:gateway}"}
      """
    Then the response status code should be 201
    And I store the JSON response field "buildId" as "deployedBuild"
    And I store the JSON response field "deploymentId" as "deployment"
    When I send a "GET" request to the control plane at "/<collection>/${CTX:<artifact>}/builds"
    Then the JSON response array "list" should contain an item with "buildId" equal to "${CTX:deployedBuild}"
    When I send a "GET" request to the control plane at "/<collection>/${CTX:<artifact>}/deployments/${CTX:deployment}"
    Then the response status code should be 200
    And the JSON response field "buildId" should be "${CTX:deployedBuild}"
    When I send a "POST" request to the control plane at "/<collection>/${CTX:<artifact>}/deployments" with body:
      """
      {"name": "${UNIQUE:bld-dep}", "base": "build", "buildId": "${CTX:deployedBuild}", "gatewayId": "${CTX:gateway}"}
      """
    Then the response status code should be 201
    And the JSON response field "buildId" should be "${CTX:deployedBuild}"

    Examples:
      | collection    | artifact | code                                  |
      | rest-apis     | shared   | REST_API_DEPLOYMENT_VALIDATION_FAILED |
      | mcp-proxies   | shared   | MCP_PROXY_DEPLOYMENT_VALIDATION_FAILED |
      | graphql-apis  | graphql  | GRAPHQL_API_DEPLOYMENT_VALIDATION_FAILED |
      | llm-providers | provider | LLM_PROVIDER_DEPLOYMENT_VALIDATION_FAILED |
      | llm-proxies   | proxy    | LLM_PROXY_DEPLOYMENT_VALIDATION_FAILED |

  Scenario: A build in use by a deployment survives pruning, and a limit of held builds refuses another
    Given I generate a unique resource name from "bld-gw-a" and store it as "gatewayA"
    And I generate a unique resource name from "bld-gw-b" and store it as "gatewayB"
    And I generate a unique resource name from "bld-gw-c" and store it as "gatewayC"
    And I generate a unique resource name from "bld-gw-d" and store it as "gatewayD"
    And I generate a unique resource name from "bld-gw-e" and store it as "gatewayE"
    And I register a gateway "${CTX:gatewayA}" on the control plane
    And I register a gateway "${CTX:gatewayB}" on the control plane
    And I register a gateway "${CTX:gatewayC}" on the control plane
    And I register a gateway "${CTX:gatewayD}" on the control plane
    And I register a gateway "${CTX:gatewayE}" on the control plane
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:shared}/builds"
    Then the response status code should be 201
    And I store the JSON response field "buildId" as "held"
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:shared}/deployments" with body:
      """
      {"name": "${UNIQUE:bld-held}", "base": "build", "buildId": "${CTX:held}", "gatewayId": "${CTX:gatewayA}"}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:shared}/builds"
    Then the response status code should be 201
    And I store the JSON response field "buildId" as "unheld"
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:shared}/builds"
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:shared}/builds"
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:shared}/builds"
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:shared}/builds"
    Then the response status code should be 201
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:shared}/builds"
    Then the JSON response field "count" should be 5
    And the JSON response array "list" should contain an item with "buildId" equal to "${CTX:held}"
    And the JSON response array "list" should not contain an item with "buildId" equal to "${CTX:unheld}"
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:shared}/deployments" with body:
      """
      {"name": "${UNIQUE:bld-b}", "base": "current", "gatewayId": "${CTX:gatewayB}"}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:shared}/deployments" with body:
      """
      {"name": "${UNIQUE:bld-c}", "base": "current", "gatewayId": "${CTX:gatewayC}"}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:shared}/deployments" with body:
      """
      {"name": "${UNIQUE:bld-d}", "base": "current", "gatewayId": "${CTX:gatewayD}"}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:shared}/deployments" with body:
      """
      {"name": "${UNIQUE:bld-e}", "base": "current", "gatewayId": "${CTX:gatewayE}"}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:shared}/builds"
    Then the response status code should be 409
    And the JSON response field "code" should be "BUILD_LIMIT_REACHED"
    And the JSON response field "message" should contain "5 builds"
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:shared}/deployments" with body:
      """
      {"name": "${UNIQUE:bld-over}", "base": "current", "gatewayId": "${CTX:gatewayA}"}
      """
    Then the response status code should be 409
    And the JSON response field "code" should be "BUILD_LIMIT_REACHED"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:shared}/deployments"
    Then the JSON response field "count" should be 5
